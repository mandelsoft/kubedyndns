# kubedyndns

## Name

*kubedyndns* - enables serving dns records from kubernetes resources.

## Description

This plugin reads records for the served zones from resources maintained 
in a kubernetes cluster.

This plugin can only be used once per Server Block.

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
* `Primary`: the server acts as primary server of a single (specified)
  domain using entry objects in a single namespace. There must be an
  appropriate `HostedZone` object (attribute `zoneobject`) in this namespace and considered
  entry object must declare this zone object as their zone.
  NS record entries can be used to delegate subdomains to external
  nameservers. `HostedZone` objects which transitively refer to the
  initial zone object can be used to provide delegation by the same
  dataplane.

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