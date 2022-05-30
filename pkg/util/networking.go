package util

// Copyright 2022 Thomas Pilz

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// 	http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import (
	"fmt"
	"net"
	"net/http"
)

type HttpResponse struct {
	Resp *http.Response
	Err  error
}

func HttpGetAsync(url string, rc chan HttpResponse) {
	resp, err := http.Get(url)
	rc <- HttpResponse{
		resp,
		err,
	}
}

// Tries to resolve an IP address for a given domain
func ResolveHostnameToIp(domain string) (net.IP, error) {
	ips, err := net.LookupIP(domain)
	if err != nil || len(ips) < 1 {
		return nil, fmt.Errorf("Could not get IP for %v: %v\n", domain, err)
	}
	return ips[0], nil
}

func GetIp(host string) (net.IP, error) {
	// if parsing is successful, what was passed in already is a valid IP address
	ip := net.ParseIP(host)
	if ip == nil {
		// ParseIP returns nil if the it is no valid IP
		// try to resolve an IP for the given domain
		var err error
		ip, err = ResolveHostnameToIp(host)
		if err != nil {
			return nil, fmt.Errorf("remote host %v could not be resolved", host)
		}
	}
	return ip, nil
}
