package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
	"time"
)

type IngressList struct {
	APIVersion string `yaml:"apiVersion"`
	Items      []struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			Annotations struct {
				AviProxy                   string `yaml:"avi_proxy"`
				MetaHelmShReleaseName      string `yaml:"meta.helm.sh/release-name"`
				MetaHelmShReleaseNamespace string `yaml:"meta.helm.sh/release-namespace"`
			} `yaml:"annotations"`
			CreationTimestamp time.Time `yaml:"creationTimestamp"`
			Generation        int       `yaml:"generation"`
			Labels            struct {
				App                      string `yaml:"app"`
				AppKubernetesIoManagedBy string `yaml:"app.kubernetes.io/managed-by"`
				AppdAgent                string `yaml:"appdAgent"`
				Domain                   string `yaml:"domain"`
			} `yaml:"labels"`
			Name            string `yaml:"name"`
			Namespace       string `yaml:"namespace"`
			ResourceVersion string `yaml:"resourceVersion"`
			SelfLink        string `yaml:"selfLink"`
			UID             string `yaml:"uid"`
		} `yaml:"metadata"`
		Spec   IngressSpec `yaml:"spec"`
		Status struct {
			LoadBalancer struct {
				Ingress []struct {
					Hostname string `yaml:"hostname"`
					IP       string `yaml:"ip"`
				} `yaml:"ingress"`
			} `yaml:"loadBalancer"`
		} `yaml:"status"`
	} `yaml:"items"`
}

type IngressSpec struct {
	Rules []IngressRule `yaml:"rules"`
}

type IngressRule struct {
	Host string   `yaml:"host"`
	HTTP RuleHTTP `yaml:"http"`
}

type RuleHTTP struct {
	Paths []IngressPath `yaml:"paths"`
}

type IngressPath struct {
	Backend  Backend `yaml:"backend"`
	Path     string  `yaml:"path"`
	PathType string  `yaml:"pathType"`
}

type Backend struct {
	ServiceName string `yaml:"serviceName"`
	ServicePort string `yaml:"servicePort"`
}

type aviProxy struct {
	Name           string
	NameSpace      string
	Rules          []IngressRule
	Virtualservice struct {
		Fqdn                  string `json:"fqdn"`
		ApplicationProfileRef string `json:"application_profile_ref"`
		Services              []struct {
			Port      int  `json:"port"`
			EnableSsl bool `json:"enable_ssl"`
		} `json:"services"`
		SslKeyAndCertificateRefs []string `json:"ssl_key_and_certificate_refs"`
		SslProfileRef            string   `json:"ssl_profile_ref"`
		VsVipRef                 string   `json:"vsvip_ref"`
		SEGroupRef               string   `json:"se_group_ref"`
		TenantRef                string   `json:"tenant_ref"`
		EWPlacement              bool     `json:"east_west_placement"`
		WafPolicyRef             string   `json:"waf_policy_ref"`
	} `json:"virtualservice"`
	Pool struct {
		HealthMonitorRefs []string `json:"health_monitor_refs"`
		LbAlgo            string   `json:"lb_algorithm"`
		LbAlgoHash        string   `json:"lb_algorithm_hash"`
		SslProfileRef     string   `json:"ssl_profile_ref"`
	} `json:"pool"`
	GslbService struct {
		Name              string   `json:"name"`
		DomainNames       []string `json:"domain_names"`
		HealthMonitorRefs []string `json:"health_monitor_refs"`
		Ttl               int      `json:"ttl"`
	} `json:"gslbservice"`
}

var hostcount int
var httpcount int
var gslbcount int

func (a aviProxy) CreateHostRule() {
	//APP Profile Logic
	re := regexp.MustCompile(`=(.*)`)
	ap := re.FindString(a.Virtualservice.ApplicationProfileRef)
	ap = strings.Trim(ap, "=")
	//ap = fmt.Sprintf("%q", ap)
	////////////////////

	cr := ""
	sslp := ""
	//SSLKeyCert Logic
	if len(a.Virtualservice.SslKeyAndCertificateRefs) > 0 {
		cr = re.FindString(a.Virtualservice.SslKeyAndCertificateRefs[0]) //TODO: Expand for multiple certs
		cr = strings.Trim(cr, "=")
		//cr = fmt.Sprintf("%q", cr)
		sslp = re.FindString(a.Virtualservice.SslProfileRef) //TODO: Expand for multiple certs
		sslp = strings.Trim(sslp, "=")
	}
	////////////////////

	//WafProfile Logic
	wp := re.FindString(a.Virtualservice.WafPolicyRef)
	wp = strings.Trim(wp, "=")

	hostrule := HostRule{
		APIVersion: "ako.vmware.com/v1alpha1",
		Kind:       "HostRule",
		Metadata: Metadata{
			Name:			a.Name + "-hostrule",
			Namespace:		a.NameSpace,
			Annotations: 	Annotations {
				MetaHelmShReleaseName:		a.Name,
				MetaHelmShReleaseNamespace:	a.NameSpace,
			},
			Labels: Labels {
				AppKubernetesIoManagedBy: "Helm",
			},
		},
		Spec: HostSpec{
			Virtualhost: Virtualhost{
				Fqdn: a.Virtualservice.Fqdn,
				TLS: HostTLS{
					SslKeyCertificate: SslKeyCertificate{
						Name: cr,
						Type: "ref",
					},
					SslProfile:  sslp,
					Termination: "edge", //Only edge is supported as of AKO 1.5, ReEncrypt is an HTTPRule
				},
				HTTPPolicy: HTTPPolicy{
					PolicySets: nil, //: These must be precreated in the controller
					// TODO: Parse from avi_proxy "httppolicyset" key
					Overwrite: false,
				},
				WafPolicy:          wp,
				ApplicationProfile: ap,
				AnalyticsProfile:   "",
				errorPageProfile:   "",
			},
		},
	}

	if len(a.GslbService.DomainNames) > 0 {
		hostrule.Spec.Virtualhost.Gslb.Fqdn = a.GslbService.DomainNames[0]
	} else {
		log.Printf("GSLB domain name not found for ingress %s, skipping GSLB FQDN in HostRule\n", a.Name)
		return
	}

	//Marshall Yaml
	data, err := yaml.Marshal(&hostrule)
	if err != nil {
		log.Fatal(err)
	}

	//Write to file
	err = ioutil.WriteFile("./hostrules/"+hostrule.Metadata.Name+hostrule.Metadata.Namespace+".yaml", data, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(hostrule.Metadata.Name + ".yaml created")
	hostcount = hostcount + 1
	//log.Println("HostRule Count: ", hostcount)

}

func (a aviProxy) CreateHTTPRule() {

	//Rules/Path Logic
	//Right now I rely on fqdn being present in avi_proxy,
	//this will be need to reworked in the future and logic needs to be added if the fqdn is not present so it can be
	//inferred from the ingress spec

	path := ""
	var paths []string
	for _, r := range a.Rules {
		if a.Virtualservice.Fqdn == r.Host { // Matching avi_proxy fqdn against spec host fqdn
			// TODO: Add logic for cases where fqdn is not in avi proxy
			//fmt.Println(len(r.HTTP.Paths))
			if len(r.HTTP.Paths) > 1 {
				for _, p := range r.HTTP.Paths {
					paths = append(paths, p.Path)
				}
			} else if len(r.HTTP.Paths) == 1 {
				path = r.HTTP.Paths[0].Path
			} else {
				// No Paths so return
				fmt.Printf("No path found for %s-httprule so skipping\n", a.Name)
				return
			}
		}
	}
	////////////////////

	//fmt.Println(path)

	//Build HTTPPath Slice/regex
	var httppaths []HttpPath
	var hmlist []string
	re := regexp.MustCompile(`=(.*)`)

	//build hm list
	if len(a.Pool.HealthMonitorRefs) >= 1 {
		//fmt.Println(a.Pool.HealthMonitorRefs)
		for _, hm := range a.Pool.HealthMonitorRefs {
			hm = re.FindString(hm)
			hm = strings.Trim(hm, "=")
			hmlist = append(hmlist, hm)
		}
	}

	// Parse LB Algo and Hash
	lbdata := LoadBalancerPolicy{}
	if a.Pool.LbAlgo != "" {
		lbdata.Algorithm = a.Pool.LbAlgo
	}

	if a.Pool.LbAlgoHash != "" {
		lbdata.Hash = a.Pool.LbAlgoHash
	}

	////////////////////

	//TODO: Parse re-encrypt information

	///////////////
	if len(paths) > 0 {
		for _, p := range paths {
			httppath := HttpPath{
				Target:             p,
				HealthMonitors:     hmlist,
				LoadBalancerPolicy: lbdata,
				TLS:                HttpTLS{},
			}
			httppaths = append(httppaths, httppath)
		}
	} else {
		httppath := HttpPath{
			Target:             path,
			HealthMonitors:     hmlist,
			LoadBalancerPolicy: lbdata,
			TLS:                HttpTLS{},
		}
		httppaths = append(httppaths, httppath)
	}

	//Build HttpRule
	httprule := HttpRule{
		APIVersion: "ako.vmware.com/v1alpha1",
		Kind:       "HTTPRule",
		Metadata: Metadata{
			Name:      a.Name + "-httprule",
			Namespace: a.NameSpace,
			Annotations: 	Annotations {
				MetaHelmShReleaseName: 		a.Name,
				MetaHelmShReleaseNamespace:	a.NameSpace,
			},
			Labels: Labels {
				AppKubernetesIoManagedBy: "Helm",
			},
		},
		Spec: HttpSpec{
			Fqdn:  a.Virtualservice.Fqdn,
			Paths: httppaths,
		},
	}

	//Marshall Yaml
	data, err := yaml.Marshal(&httprule)
	if err != nil {
		log.Fatal(err)
	}

	//Write to file
	err = ioutil.WriteFile("./httprules/"+httprule.Metadata.Name+httprule.Metadata.Namespace+".yaml", data, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(httprule.Metadata.Name + ".yaml created")
	httpcount = httpcount + 1
	//log.Println("HTTPRule Count: ", httpcount)

}

func (a aviProxy) CreateGSLBRule() {

	fq := ""
	//FQDN Logic
	if len(a.GslbService.DomainNames) > 0 {
		fq = a.GslbService.DomainNames[0]
	} else {
		log.Printf("GSLB domain name not found for ingress %s, skipping\n", a.Name)
		return
	}
	////////////////////

	//HM Logic
	var hmlist []string
	re := regexp.MustCompile(`=(.*)`)

	//build hm list
	if len(a.GslbService.HealthMonitorRefs) > 0 {
		for _, hm := range a.GslbService.HealthMonitorRefs {
			hm = re.FindString(hm)
			hm = strings.Trim(hm, "=")
			hmlist = append(hmlist, hm)
		}
	}
	////////////////////

	gslbrule := GSLBHostRule{
		APIVersion: "amko.vmware.com/v1alpha1",
		Kind:       "GSLBHostRule",
		Metadata: Metadata{
			Name:      a.Name + "-gslbrule",
			Namespace: a.NameSpace,
			Annotations: 	Annotations {
				MetaHelmShReleaseName:		a.Name,
				MetaHelmShReleaseNamespace:	a.NameSpace,
			},
			Labels: Labels {
				AppKubernetesIoManagedBy: "Helm",
			},
		},
		Spec: GSLBHostSpec{
			Fqdn:                  fq,
			SitePersistence:       SitePersistence{},
			ThirdPartyMembers:     nil, //Not in avi_proxy
			HealthMonitorRefs:     hmlist,
			TrafficSplit:          nil, //Not in avi_proxy
			TTL:                   a.GslbService.Ttl,
			PoolAlgorithmSettings: PoolAlgorithmSettings{},
		},
	}

	data, err := yaml.Marshal(&gslbrule)
	if err != nil {
		log.Fatal(err)
	}

	//Write to file
	err = ioutil.WriteFile("./gslbrules/"+gslbrule.Metadata.Name+gslbrule.Metadata.Namespace+".yaml", data, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(gslbrule.Metadata.Name + ".yaml created")
	gslbcount = gslbcount + 1
	//log.Println("GSLBRule Count: ", gslbcount)
}

// HostRule Models

type HostRule struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       HostSpec `yaml:"spec"`
}

type Metadata struct {
	Name      	string		`yaml:"name"`
	Namespace 	string		`yaml:"namespace"`
	Annotations	Annotations	`yaml:"annotations"`
	Labels		Labels		`yaml:"labels"`
}

type Annotations struct {
	MetaHelmShReleaseName      string `yaml:"meta.helm.sh/release-name"`
	MetaHelmShReleaseNamespace string `yaml:"meta.helm.sh/release-namespace"`
}

type Labels struct {
	AppKubernetesIoManagedBy string `yaml:"app.kubernetes.io/managed-by"`
}

type HostSpec struct {
	Virtualhost Virtualhost `yaml:"virtualhost"`
}

type Virtualhost struct {
	Fqdn               string     `yaml:"fqdn"`
	Gslb   			   HostGSLB   `yaml:"gslb"`
	TLS                HostTLS    `yaml:"tls,omitempty"`
	HTTPPolicy         HTTPPolicy `yaml:"httpPolicy,omitempty"`
	WafPolicy          string     `yaml:"wafPolicy,omitempty"`
	ApplicationProfile string     `yaml:"applicationProfile,omitempty"`
	AnalyticsProfile   string     `yaml:"analyticsProfile,omitempty"`
	errorPageProfile   string     `yaml:"errorPageProfile,omitempty"`
}

type HostGSLB struct {
	Fqdn string `yaml:"fqdn"`
}

type HostTLS struct {
	SslKeyCertificate SslKeyCertificate `yaml:"sslKeyCertificate,omitempty"`
	SslProfile        string            `yaml:"sslProfile,omitempty"`
	Termination       string            `yaml:"termination,omitempty"`
}

type SslKeyCertificate struct {
	Name string `yaml:"name,omitempty"`
	Type string `yaml:"type,omitempty"`
}

type HTTPPolicy struct {
	PolicySets []string `yaml:"policySets,omitempty"`
	Overwrite  bool     `yaml:"overwrite,omitempty"`
}

// HttpRule Models
type HttpRule struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata,omitempty"`
	Spec       HttpSpec `yaml:"spec,omitempty"`
}

type HttpSpec struct {
	Fqdn  string     `yaml:"fqdn,omitempty"`
	Paths []HttpPath `yaml:"paths,omitempty"`
}

type HttpPath struct {
	Target             string             `yaml:"target,omitempty"`         //example /foo
	HealthMonitors     []string           `yaml:"healthMonitors,omitempty"` //example my-hm-1
	LoadBalancerPolicy LoadBalancerPolicy `yaml:"loadBalancerPolicy,omitempty"`
	TLS                HttpTLS            `yaml:"tls,omitempty"`
}

type LoadBalancerPolicy struct {
	Algorithm string `yaml:"algorithm,omitempty"` //example LB_ALGORITHM_CONSISTENT_HASH
	Hash      string `yaml:"hash,omitempty"`      //example LB_ALGORITHM_CONSISTENT_HASH_SOURCE_IP_ADDRESS
}

type HttpTLS struct {
	Type          string `yaml:"type,omitempty"`
	SslProfile    string `yaml:"sslProfile,omitempty"`
	DestinationCA string `yaml:"destinationCA,omitempty"`
}

// GSLBHostRule Models
type GSLBHostRule struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   Metadata     `yaml:"metadata,omitempty"`
	Spec       GSLBHostSpec `yaml:"spec,omitempty"`
}

type GSLBHostSpec struct {
	Fqdn                  string                `yaml:"fqdn"`
	SitePersistence       SitePersistence       `yaml:"sitePersistence,omitempty"`
	ThirdPartyMembers     []ThirdPartyMember    `yaml:"thirdPartyMembers,omitempty"`
	HealthMonitorRefs     []string              `yaml:"healthMonitorRefs,omitempty"`
	TrafficSplit          []TrafficSplit        `yaml:"trafficSplit,omitempty"`
	TTL                   int                   `yaml:"ttl,omitempty"`
	PoolAlgorithmSettings PoolAlgorithmSettings `yaml:"poolAlgorithmSettings,omitempty"`
}

type SitePersistence struct {
	Enabled    bool   `yaml:"enabled,omitempty"`
	ProfileRef string `yaml:"profileRef,omitempty"`
}

type ThirdPartyMember struct {
	Site string `yaml:"site,omitempty"`
	Vip  string `yaml:"vip,omitempty"`
}

type TrafficSplit struct {
	Cluster string `yaml:"cluster,omitempty"`
	Weight  int    `yaml:"weight,omitempty"`
}

type PoolAlgorithmSettings struct {
	LbAlgorithm string `yaml:"lbAlgorithm,omitempty"`
}
