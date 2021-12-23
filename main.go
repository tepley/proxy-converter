package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
)


func main() {

	filePtr := flag.String("file", "ingress.yaml", "Ingress Yaml filename to convert")

	flag.Parse()

	//Read Ingress Yaml File
	var ingress IngressList
	yamlFile, err := ioutil.ReadFile(*filePtr)
	if err != nil {
		log.Fatal(err)
	}

	//Unmarshal Ingress YAML list
	err = yaml.Unmarshal(yamlFile, &ingress)
	if err != nil {
		panic(err)
	}

	var services []aviProxy
	for _, i := range ingress.Items {
		//Unmarshal Avi_Proxy Annotations
		var service aviProxy
		err = json.Unmarshal([]byte(i.Metadata.Annotations.AviProxy), &service)
		if err != nil {
			log.Fatalf("Couldn't decode Avi_Proxy Data for %s, Error: %s\n", service.Name, err.Error())
		}
		service.Name = i.Metadata.Name
		service.NameSpace = i.Metadata.Namespace
		service.Rules = i.Spec.Rules
		services = append(services, service)
	}
	fmt.Println()

	//bar := progressbar.Default(int64(len(services)))

	//Create dir if it doesnt exist
	err = os.MkdirAll("./hostrules", os.ModePerm)
	if err != nil {
		panic("Couldn't make hostrule directory")
	}
	err = os.MkdirAll("./httprules", os.ModePerm)
	if err != nil {
		panic("Couldn't make hostrule directory")
	}
	err = os.MkdirAll("./gslbrules", os.ModePerm)
	if err != nil {
		panic("Couldn't make hostrule directory")
	}

	//fmt.Println("Processing CRDs")
	for _, s := range services {

		s.CreateHostRule()
		s.CreateHTTPRule()
		s.CreateGSLBRule()
		//bar.Add(1)
	}

	fmt.Println("Total HostRule CRDs created: ", hostcount)
	fmt.Println("Total HTTPRule CRDs created: ", httpcount)
	fmt.Println("Total GSLBRule CRDs created: ", gslbcount)

}
