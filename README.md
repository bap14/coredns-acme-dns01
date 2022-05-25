# CoreDNS Acme DNS-01 Challenge Tool

A simple program to generate CoreDNS zone files and add / remove records in
response to DNS-01 ACME commands.

Developed for use with Traefik as a way to allow local SSL generation with
StepCA. Supports creating initial zone files, adding records, and cleaning up
records once the challenge is complete.

## Options

Certain options can be specified as environment variables to customize the zone
file.

 - *COREDNS_ACME_NS* Default: `172.19.0.53`
   Specify the nameserver IPv4 address to be used in the zone file. This must be
   resolvable by Traefik, and StepCA, to complete the DNS-01 challenge.
 - *COREDNS_ACME_OUT* Default: `/coredns/zones.d`
   Specify the output directory for generated zone files, or zone files that are
   to be updated.
 - *COREDNS_ACME_DEBUG* Default: `false`
   Enable extra output to help track down issues
