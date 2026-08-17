// Copyright 2023 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package object

import (
	"fmt"
	"strings"

	"github.com/apache/casbin-gateway/auth"
	"github.com/apache/casbin-gateway/conf"
)

var siteMap = map[string]*Site{}
var certMap = map[string]*Cert{}
var healthCheckNeededDomains []string

func InitSiteMap() {
	err := refreshSiteMap()
	if err != nil {
		panic(err)
	}
}

func getCasdoorCertMap() (map[string]*auth.Cert, error) {
	certs, err := auth.GetCerts()
	if err != nil {
		return nil, fmt.Errorf("auth.GetCerts() error: %s", err.Error())
	}

	res := map[string]*auth.Cert{}
	for _, cert := range certs {
		res[cert.Name] = cert
	}
	return res, nil
}

func getCasdoorApplicationMap() (map[string]*auth.Application, error) {
	// Sites are only bound to a Casdoor application when Casdoor is configured.
	if !conf.IsCasdoorAvailable() {
		return nil, nil
	}

	casdoorCertMap, err := getCasdoorCertMap()
	if err != nil {
		return nil, err
	}

	applications, err := auth.GetOrganizationApplications()
	if err != nil {
		return nil, fmt.Errorf("auth.GetOrganizationApplications() error: %s", err.Error())
	}

	res := map[string]*auth.Application{}
	for _, application := range applications {
		if application.Cert != "" {
			if cert, ok := casdoorCertMap[application.Cert]; ok {
				application.CertObj = cert
			}
		}

		res[application.Name] = application
	}
	return res, nil
}

func refreshSiteMap() error {
	applicationMap, err := getCasdoorApplicationMap()
	if err != nil {
		fmt.Println(err)
	}

	newSiteMap := map[string]*Site{}
	newHealthCheckNeededDomains := make([]string, 0)
	sites, err := GetGlobalSites()
	if err != nil {
		return err
	}

	certMap, err = getCertMap()
	if err != nil {
		return err
	}

	for _, site := range sites {
		if applicationMap != nil {
			if site.CasdoorApplication != "" && site.ApplicationObj == nil {
				if v, ok2 := applicationMap[site.CasdoorApplication]; ok2 {
					site.ApplicationObj = v
				}
			}
		}

		if site.Domain != "" && site.PublicIp == "" {
			go func(site *Site) {
				site.PublicIp = resolveDomainToIp(site.Domain)
				_, err = UpdateSiteNoRefresh(site.GetId(), site)
				if err != nil {
					fmt.Printf("UpdateSiteNoRefresh() error: %v\n", err)
				}
			}(site)
		}

		newSiteMap[strings.ToLower(site.Domain)] = site
		if !shouldStopHealthCheck(site) {
			newHealthCheckNeededDomains = append(newHealthCheckNeededDomains, strings.ToLower(site.Domain))
		}
		for _, domain := range site.OtherDomains {
			if domain != "" {
				newSiteMap[strings.ToLower(domain)] = site
			}
		}
	}

	siteMap = newSiteMap
	healthCheckNeededDomains = newHealthCheckNeededDomains
	return nil
}

func GetSiteByDomain(domain string) *Site {
	if site, ok := siteMap[strings.ToLower(domain)]; ok {
		return site
	} else {
		return nil
	}
}
