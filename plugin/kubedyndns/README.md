# kubedyndns

## Name

*kubedyndns* - enables serving dns records from kubernetes resources.

## Description

This plugin reads records for the served zones from resources maintained 
in a kubernetes cluster.

This plugin can only be used once per Server Block if no namespace is declared.
If multiple instance are configured only one kubernetes cluster access can be configured 
as part of the first plugin configuration in a server block.

## Syntax

~~~
kubedyndns ZONE [ZONES...]
~~~
The arguments specify all the zones the plugin should be authoritative for.
It will filter the kubernetes resources according to the given zones.

```
kubedyndns [ZONES...] {
    endpoint URL
    tls CERT KEY CACERT
    kubeconfig KUBECONFIG CONTEXT
    namespaces NAMESPACE...
    labels EXPRESSION
    ttl TTL
    fallthrough [ZONES...]
}
```

* `mode` specifies the way dns entry resources are interpreted (see below)
* `zoneobject` specifies the object in the cluster describing the
  hosted zone served in `Primary`mode.
* `transitive` can be used to enable a transitive handling of the zone object.
  Values can be `true`or `false`(default `false` if not set at all and `true` if used without argument)
* `endpoint` specifies the **URL** for a remote k8s API endpoint.
   If omitted, it will connect to k8s in-cluster using the cluster service account.
* `tls` **CERT** **KEY** **CACERT** are the TLS cert, key and the CA cert file names for remote k8s connection.
   This option is ignored if connecting in-cluster (i.e. endpoint is not specified).
* `kubeconfig` **KUBECONFIG** **CONTEXT** authenticates the connection to a remote k8s cluster using a kubeconfig file. It supports TLS, username and password, or token-based authentication. This option is ignored if connecting in-cluster (i.e., the endpoint is not specified).
* `namespaces` **NAMESPACE [NAMESPACE...]** only exposes the k8s namespaces listed.
   If this option is omitted all namespaces are exposed
* `labels` **EXPRESSION** only exposes the records for Kubernetes objects that match this label selector.
   The label selector syntax is described in the
   [Kubernetes User Guide - Labels](https://kubernetes.io/docs/user-guide/labels/). An example that
   only exposes objects labeled as "application=nginx" in the "staging" or "qa" environments, would
   use: `labels environment in (staging, qa),application=nginx`.
* `ttl` allows you to set a custom TTL for responses. The default is 5 seconds.  The minimum TTL allowed is
  0 seconds, and the maximum is capped at 3600 seconds. Setting TTL to 0 will prevent records from being cached.
* `fallthrough` **[ZONES...]** If a query for a record in the zones for which the plugin is authoritative
  results in NXDOMAIN, normally that is what the response will be. However, if you specify this option,
  the query will instead be passed on down the plugin chain, which can include another plugin to handle
  the query. If **[ZONES...]** is omitted, then fallthrough happens for all zones for which the plugin
  is authoritative. If specific zones are listed (for example `in-addr.arpa` and `ip6.arpa`), then only
  queries for those zones will be subject to fallthrough.

## Resource

The plugin scans for resource with kind `CoreDNSEntry` in api group `coredns.mandelsoft.org/v1alpha1`.
(The CRD can be found [here](../../apis/coredns/crds/coredns.mandelsoft.org_corednsentries.yaml).)
It features the following fields:

```yaml
kind: CoreDNSEntry
apiVersion: coredns.mandelsoft.org/v1alpha1
  
metadata:
  name: test 
  namespace: default
spec:
  dnsNames:
  - test.my.domain
  A:
  - 8.8.8.8
# AAAA:
# - ipv6 address
# CNAME: my.cname
  TXT:
  - this is a dns server
  SRV:
    service: dns
    records:
      - port: 53
        protocol: UDP
        host: dns.google
```

## Modes

The plugin supports multiple ways to interpret the settings in a core dns entry 
object:
* `FilterByZones`: dns names are filtered against the zones declared in the plugin block
* `Subdomains`: namespace and zone is used to complete the served dns names. Only
  one authoritative zone is possible.
* `Primary`: the server acts as primary server of a tree (specified)
  of locally defined authoritative zones using entry and zone objects in a single namespace. There must be an
  appropriate `HostedZone` root object (attribute `zoneobject`) in this namespace.
  It considers
  entry objects declaring this zone object as their zone. if the `transitive` attribute is set to true all transitively nested hosted zone objects are handled, also.

  NS record entries can be used to delegate subdomains to external
  nameservers. `HostedZone` objects which transitively refer to the
  initial zone object can be used to provide delegation by the same
  dataplane.

### Primary Mode

In `Primary`  mode the plugin serves hosted zones defined by a `HostedZone`
resource. It considers all entry objecxts referring to this zone object.
`NS` entry objects or other zone objects can be used to describe
delegated zones. If zone objects are used, those nested zones can be
handled by the same plugin instance if the `transitive` attribute is set to `true`.

Every zone object can define more than one domain, which such provide the same 
set of sub-domains. The root object must declare fully qualified domain names
and no parent reference. A nested zone must declare its parent tone object and
the declared domain names must be specified relative to the parent.

The plugin scans for a zone resource with kind `HostedZone` in api group `coredns.mandelsoft.org/v1alpha1`.
(The CRD can be found [here](../../apis/coredns/crds/coredns.mandelsoft.org_hostedzones.yaml).)

```yaml
kind: HostedZone
apiVersion: coredns.mandelsoft.org/v1alpha1
metadata:
  name: test
  namespace: default
spec:
  domainNames:
  - test.mandelsoft.org
  - test.mandelsoft.de
  email: uwe.krueger@mandelsoft.de
  refresh: 7200
  retry: 3600
  expire: 1209600
  minimumTTL: 3600
```

A sub-domain the looks like this

```yaml
kind: HostedZone
apiVersion: coredns.mandelsoft.org/v1alpha1
metadata:
  name: a-nested-test
  namespace: default
spec:
  parentRef: test
  domainNames:
    - a.nested
  email: uwe.krueger@mandelsoft.de
  refresh: 7200
  retry: 3600
  expire: 1209600
  minimumTTL: 3600
```

If transitive mode is set the following zones are handled
- test.mandelsoft.de
- test.mandelsoft.org 
- a.nested.test.mandelsoft.de
- a.nested.test.mandelsoft.org

Entry objects must declare domain names relative to its declared zones object.

```yaml
apiVersion: coredns.mandelsoft.org/v1alpha1
metadata:
  name: demo
  namespace: default
spec:
  zoneRef: test
  dnsNames:
    - demo
  A:
    - 8.8.8.8
  TXT:
    - this is a test server
  SRV:
    service: dns
    records:
      - port: 53
        protocol: UDP
        priority: 10
        weight: 100
        host: dns.google.  # fqdn
      - port: 53
        protocol: UDP
        priority: 10
        weight: 100
        host: dns # relative dn
```

Such an entry the resolved the following names:
- demo.test.mandelsoft.org
- demo.test.mandelsoft.de

If multiple zones are served the plugin should be configured with zone `.`.
With this, the served zones are completely defined by the specified zone object.
If a domain names are configured, the declaration in the zone object is used relative
to those domains.

### FilterSubDomains

The server handles all entry object mathing the domain(s) declared in the
plugin block, which do not have a `zoneRef` attribute. The domain declared domain names
must be fully qualified.

### FilterSubDomains

The server serves all entries without a zone ref with all zones defined
in the plugin block. Therefore, the domain names declared in the entry objects must
be relative.

## Ready

This plugin reports readiness to the ready plugin. This will happen after it has synced to the
Kubernetes API.

## Examples

Handle all queries in the `my.domain` zone. Connect to Kubernetes in-cluster. Also handle all
`in-addr.arpa` `PTR` requests` .

~~~ txt
.:1053 {
    kubedyndns my.domain in-addr.arpa
}
~~~

Connect to Kubernetes with CoreDNS running outside the cluster:

~~~ txt
.:1053 {
  kubedyndns my.domain {
      endpoint https://k8s-endpoint:8443
      tls cert key cacert
  }
}
~~~