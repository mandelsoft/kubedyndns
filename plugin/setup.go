package kubedyndns

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/upstream"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"       // pull this in here, because we want it excluded if plugin.cfg doesn't have k8s
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"      // pull this in here, because we want it excluded if plugin.cfg doesn't have k8s
	_ "k8s.io/client-go/plugin/pkg/client/auth/openstack" // pull this in here, because we want it excluded if plugin.cfg doesn't have k8s
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

const pluginName = "kubedyndns"

var log = clog.NewWithPlugin(pluginName)

func init() { plugin.Register(pluginName, setup) }

func setup(c *caddy.Controller) error {
	klog.SetOutput(os.Stdout)

	k, err := parse(c)
	if err != nil {
		return plugin.Error(pluginName, err)
	}

	err = k.InitKubeCache(context.Background())
	if err != nil {
		return plugin.Error(pluginName, err)
	}

	k.RegisterKubeCache(c)

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		k.Next = next
		return k
	})

	// get locally bound addresses
	c.OnStartup(func() error {
		return nil
	})

	return nil
}

// RegisterKubeCache registers KubeCache start and stop functions with Caddy
func (k *KubeDynDNS) RegisterKubeCache(c *caddy.Controller) {
	c.OnStartup(func() error {
		go k.APIConn.Run()

		timeout := time.After(5 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				if k.APIConn.HasSynced() {
					return nil
				}
			case <-timeout:
				return nil
			}
		}
	})

	c.OnShutdown(func() error {
		return k.APIConn.Stop()
	})
}

func parse(c *caddy.Controller) (*KubeDynDNS, error) {
	var (
		k8s *KubeDynDNS
		err error
	)

	i := 0
	for c.Next() {
		if i > 0 {
			return nil, plugin.ErrOnce
		}
		i++

		k8s, err = ParseStanza(c)
		if err != nil {
			return k8s, err
		}
	}
	return k8s, nil
}

// ParseStanza parses a kubedyndns stanza
func ParseStanza(c *caddy.Controller) (*KubeDynDNS, error) {

	k8s := New([]string{""})

	zones := c.RemainingArgs()

	if len(zones) != 0 {
		k8s.Zones = zones
		for i := 0; i < len(k8s.Zones); i++ {
			k8s.Zones[i] = plugin.Host(k8s.Zones[i]).Normalize()
		}
	} else {
		k8s.Zones = make([]string, len(c.ServerBlockKeys))
		for i := 0; i < len(c.ServerBlockKeys); i++ {
			k8s.Zones[i] = plugin.Host(c.ServerBlockKeys[i]).Normalize()
		}
	}

	k8s.primaryZoneIndex = -1
	for i, z := range k8s.Zones {
		if dnsutil.IsReverse(z) > 0 {
			continue
		}
		k8s.primaryZoneIndex = i
		break
	}

	if k8s.primaryZoneIndex == -1 {
		return nil, errors.New("non-reverse zone name must be used")
	}

	k8s.Upstream = upstream.New()

	for c.NextBlock() {
		switch c.Val() {
		case "namespaces":
			args := c.RemainingArgs()
			if len(args) > 0 {
				for _, a := range args {
					k8s.Namespaces[a] = struct{}{}
				}
				continue
			}
			return nil, c.ArgErr()
		case "endpoint":
			args := c.RemainingArgs()
			if len(args) > 0 {
				// Multiple endpoints are deprecated but still could be specified,
				// only the first one be used, though
				k8s.APIServerList = args
				if len(args) > 1 {
					return nil, fmt.Errorf("Multiple endpoints not possible")
				}
				continue
			}
			return nil, c.ArgErr()
		case "tls": // cert key cacertfile
			args := c.RemainingArgs()
			if len(args) == 3 {
				k8s.APIClientCert, k8s.APIClientKey, k8s.APICertAuth = args[0], args[1], args[2]
				continue
			}
			return nil, c.ArgErr()
		case "labels":
			args := c.RemainingArgs()
			if len(args) > 0 {
				labelSelectorString := strings.Join(args, " ")
				ls, err := meta.ParseToLabelSelector(labelSelectorString)
				if err != nil {
					return nil, fmt.Errorf("unable to parse label selector value: '%v': %v", labelSelectorString, err)
				}
				k8s.opts.labelSelector = ls
				continue
			}
			return nil, c.ArgErr()
		case "namespace_labels":
			args := c.RemainingArgs()
			if len(args) > 0 {
				namespaceLabelSelectorString := strings.Join(args, " ")
				nls, err := meta.ParseToLabelSelector(namespaceLabelSelectorString)
				if err != nil {
					return nil, fmt.Errorf("unable to parse namespace_label selector value: '%v': %v", namespaceLabelSelectorString, err)
				}
				k8s.opts.namespaceLabelSelector = nls
				continue
			}
			return nil, c.ArgErr()
		case "fallthrough":
			k8s.Fall.SetZonesFromArgs(c.RemainingArgs())
		case "ttl":
			args := c.RemainingArgs()
			if len(args) == 0 {
				return nil, c.ArgErr()
			}
			t, err := strconv.Atoi(args[0])
			if err != nil {
				return nil, err
			}
			if t < 0 || t > 3600 {
				return nil, c.Errf("ttl must be in range [0, 3600]: %d", t)
			}
			k8s.ttl = uint32(t)
		case "kubeconfig":
			args := c.RemainingArgs()
			if len(args) == 2 {
				config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
					&clientcmd.ClientConfigLoadingRules{ExplicitPath: args[0]},
					&clientcmd.ConfigOverrides{CurrentContext: args[1]},
				)
				k8s.ClientConfig = config
				continue
			}
			return nil, c.ArgErr()
		default:
			return nil, c.Errf("unknown property '%s'", c.Val())
		}
	}

	if len(k8s.Namespaces) != 0 && k8s.opts.namespaceLabelSelector != nil {
		return nil, c.Errf("namespaces and namespace_labels cannot both be set")
	}

	return k8s, nil
}
