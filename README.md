# Proxy-Converter

Author: Charlie Gutierrez  [charliegut14@gmail.com](mailto:charliegut14@gmail.com)

## About Project
This tool is used to convert Avi Proxy annotations from our legacy K8s Cloud Connector and convert them to applicable CRDs for use with AKO.



## Usage


1. Collect all ingresses from the customer environment for all namespaces

```bash
kubectl get ingress -A -o yaml > ingress.yaml
```
2. Execute the binary and pass in the yaml as a parameter, if not specified it will default to ingress.yaml
```bash
./crdgenerator -file=foo.yaml
```
3. Host rules will be generated in ./hostrules, HTTP rules will be generated in ./httprules, and GSLB rules will be generated in ./gslbrules

## Notes
This tool can currently migrate the following Avi_Proxy annotations to CRDs

Converted to HostRule CRDs
VirtualService - GSLB FQDN, Application Profile Reference, SSLKeyandCertificate Reference, WAF Policy Reference,
WIP - DS Reference

Converted to HTTPRule CRDs
Pool - HealthMonitor Reference, LoadBalancing Algorithm, LoadBalancing Algorithm Hash,
WIP - TLS Re-Encryption

Converted to GSLBHostRule CRDs (GSLB Services will be updated based on Host/Domain Name value)
GSLBService - Domain Name, HealthMonitor Reference, TTL


Unsupported Annotations
** If there are corner cases where we have working avi_proxy source examples please contact me so that I can add a the proper functionality and flags as needed. **


GSLB Third Party Members - No source in avi_proxy to convert from

GSLB Traffic Split - No source in avi_proxy to convert from

GSLB Pool Algrorithm Settings - No source in avi_proxy to convert from

Pool Group/Pool Group Ratios - No direct conversion to CRD

Analytics Profile Reference - Only an analytics policy was allowed to be specified in avi_proxy annotations and only a reference is allowd in CRDs so this is not possible.

errorPageProfile Reference - This was not allowed to be specified in avi_proxy annotations so no conversion can be performed.

HTTPPolicySet - In avi_proxy annotations users defined both the policy set reference under http_policies as well as the actual policy set within httppolicyset. As the policy set will get deleted when the avi_proxy annotation is cleaned up this would cause the migrated reference to refer to a null object.

SE Group References / VIP Network - These are now defined in a cluster scoped AviInfraSetting CRD.
To automate this we would need to parse Ingresses with SE Group References into buckets and create a single AviInfraSetting CRD for each bucket.
For K8s we then need to create an IngressClass object that references the AviInfraSettingCRD and finally update the individual ingress files. As generating ingress files is not apart of this tool this must be done manually.


## Contact
Charlie Gutierrez
[charliegut14@gmail.com](mailto:charliegut14@gmail.com)
