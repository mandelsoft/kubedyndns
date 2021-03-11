

## CoreDNS Plugin to Serve DNS Entries from Kubernetes Resource

This project provides a [CoreDNS Plugin](plugin/kubedyndns/README.md)
that watches a dedicated Kubernetes resource to serve DNS entries for
a set of zones (DNS base domains).

The plugin can be found under `plugin/kubedyndns`. The folder `cmds/coredns`
provides a complete complete coredns server including the standard plugins plus
the new `kubedyndns` plugin.

The appropriate image is `mandelsoft/coredns`
