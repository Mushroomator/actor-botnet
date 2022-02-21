package util

import "net/http"

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
