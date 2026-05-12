package handlers

import "net/http"

const multipartOverheadBytes = 1 << 20

func limitMultipartBody(w http.ResponseWriter, r *http.Request, maxFileBytes int64, maxFiles int) {
	if maxFileBytes <= 0 || maxFiles <= 0 {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFileBytes*int64(maxFiles)+multipartOverheadBytes*int64(maxFiles))
}
