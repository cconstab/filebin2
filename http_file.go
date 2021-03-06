package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/espebra/filebin2/ds"

	"github.com/dustin/go-humanize"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gorilla/mux"
)

func (h *HTTP) GetFile(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	params := mux.Vars(r)
	inputBin := params["bin"]
	// TODO: Input validation (inputBin)
	inputFilename := params["filename"]
	// TODO: Input validation (inputFilename)

	bin, found, err := h.dao.Bin().GetById(inputBin)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Failed to select bin by id %s: %s", inputBin, err.Error()), "Database error", 112, http.StatusInternalServerError)
		return
	}
	if found == false {
		h.Error(w, r, "", "The bin does not exist", 113, http.StatusNotFound)
		return
	}

	if bin.IsReadable() == false {
		h.Error(w, r, "", "This bin is no longer available.", 114, http.StatusNotFound)
		return
	}

	file, found, err := h.dao.File().GetByName(inputBin, inputFilename)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Failed to select file by bin %s and filename %s: %s", inputBin, inputFilename, err.Error()), "Database error", 115, http.StatusInternalServerError)
		return
	}
	if found == false {
		h.Error(w, r, "", fmt.Sprintf("The file %s does not exist.\n", inputFilename), 116, http.StatusNotFound)
		return
	}
	if file.IsReadable() == false {
		h.Error(w, r, "", fmt.Sprintf("The file %s is no longer available.\n", inputFilename), 117, http.StatusNotFound)
		return
	}
	if file.InStorage == false {
		h.Error(w, r, "", fmt.Sprintf("The file %s is not available.\n", inputFilename), 118, http.StatusNotFound)
		return
	}

	// Download limit
	// 0 disables the limit
	// >= 1 enforces a limit
	if h.limitFileDownloads > 0 {
		if file.Downloads >= h.limitFileDownloads {
			h.Error(w, r, "", fmt.Sprintf("The file %s has been requested too many times.\n", inputFilename), 421, http.StatusForbidden)
			return
		}
	}

	fp, err := h.s3.GetObject(inputBin, inputFilename, file.Nonce, 0, 0)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Failed to fetch object by bin %s and filename %s: %s", inputBin, inputFilename, err.Error()), "Storage error", 118, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Expires", bin.ExpiredAt.Format(http.TimeFormat))
	w.Header().Set("Last-Modified", file.UpdatedAt.Format(http.TimeFormat))
	w.Header().Set("Bin", file.Bin)
	w.Header().Set("Filename", file.Filename)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Bytes))
	//w.Header().Set("Cache-Control", "s-maxage=1")

	w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Bytes))

	if file.MD5 != "" {
		w.Header().Set("Content-MD5", file.MD5)
	}

	if file.SHA256 != "" {
		w.Header().Set("Content-SHA256", file.SHA256)
	}

	if file.Mime != "" {
		w.Header().Set("Content-Type", file.Mime)
	}

	// Handling of specific content-types
	if strings.HasPrefix(file.Mime, "video") {
		w.Header().Set("Content-Disposition", "inline")
	} else if strings.HasPrefix(file.Mime, "image") {
		w.Header().Set("Content-Disposition", "inline")
	} else if strings.HasPrefix(file.Mime, "text/plain") {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		// Tell the client to tread any other content types as attachment
		w.Header().Set("Content-Disposition", "attachment; filename=\""+file.Filename+"\"")
	}

	if err := h.dao.Bin().RegisterDownload(&bin); err != nil {
		fmt.Printf("Unable to update bin %s: %s\n", inputBin, err.Error())
	}

	if err := h.dao.File().RegisterDownload(&file); err != nil {
		fmt.Printf("Unable to update file %s in bin %s: %s\n", inputFilename, inputBin, err.Error())
	}

	fmt.Printf("Downloaded file %s (%s) from bin %s in %.3fs (%d downloads)\n", inputFilename, humanize.Bytes(file.Bytes), inputBin, time.Since(t0).Seconds(), file.Downloads)

	if bytes, err := io.Copy(w, fp); err != nil {
		fmt.Printf("The client cancelled the download of bin %s and filename %s after %s of %s\n", inputBin, inputFilename, humanize.Bytes(uint64(bytes)), humanize.Bytes(file.Bytes))
		return
	}
}

func (h *HTTP) Upload(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()

	inputBin := r.Header.Get("bin")
	inputFilename := r.Header.Get("filename")
	inputMD5 := r.Header.Get("Content-MD5")
	inputSHA256 := r.Header.Get("Content-SHA256")

	//var inputBin string
	//var inputFilename string
	//var inputMD5 string
	//var inputSHA256 string

	//if strings.HasPrefix(r.Header.Get("content-type"), "multipart/form-data") {
	//	// Multipart
	//	inputBin = r.FormValue("bin")
	//	inputFilename = r.FormValue("filename")
	//	inputMD5 = r.FormValue("Content-MD5")
	//	inputSHA256 = r.FormValue("Content-SHA256")
	//} else {
	//	inputBin = r.Header.Get("bin")
	//	inputFilename = r.Header.Get("filename")
	//	inputMD5 = r.Header.Get("Content-MD5")
	//	inputSHA256 = r.Header.Get("Content-SHA256")
	//}

	inputBytes, err := strconv.ParseUint(r.Header.Get("content-length"), 10, 64)
	if err != nil {
		h.Error(w, r, "Upload failed: Invalid content-length header", "Missing or invalid content-length header", 120, http.StatusLengthRequired)
		return
	}
	// TODO: Input validation on content-length. Between min:max.

	// Check if bin exists
	bin, found, err := h.dao.Bin().GetById(inputBin)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Failed to select bin by id %s: %s", inputBin, err.Error()), "Database error", 112, http.StatusInternalServerError)
		return
	}

	if found == false {
		// Bin does not exist, so create it here
		bin = ds.Bin{}
		bin.Id = inputBin
		bin.ExpiredAt = time.Now().UTC().Add(h.expirationDuration)
		if err := h.dao.Bin().Upsert(&bin); err != nil {
			h.Error(w, r, fmt.Sprintf("Unable to upsert bin %s: %s", inputBin, err.Error()), "Database error", 121, http.StatusInternalServerError)
			return
		}
		// TODO: Execute new bin created trigger
	}

	if bin.IsWritable() == false {
		if bin.IsExpired() {
			h.Error(w, r, fmt.Sprintf("Upload failed: Bin %s is expired", inputBin), "The bin is no longer available", 122, http.StatusMethodNotAllowed)
			return
		} else if bin.IsDeleted() {
			// Reject uploads to deleted bins
			h.Error(w, r, fmt.Sprintf("Upload failed: Bin %s is deleted", inputBin), "The bin is no longer available", 132, http.StatusMethodNotAllowed)
			return
		} else if bin.Readonly == true {
			// Reject uploads to readonly bins
			w.Header().Set("Allow", "GET, HEAD")
			h.Error(w, r, fmt.Sprintf("Rejected upload of filename %s to readonly bin %s", inputFilename, inputBin), "Uploads to locked binds are not allowed", 123, http.StatusMethodNotAllowed)
			return
		} else {
			h.Error(w, r, fmt.Sprintf("Rejected upload of filename %s to bin %s for unknown reason", inputFilename, inputBin), "Unexpected upload failure", 134, http.StatusInternalServerError)
			return
		}
	}

	info, err := h.dao.Info().GetInfo()
	if err != nil {
		fmt.Printf("Unable to GetInfo(): %s\n", err.Error())
		http.Error(w, "Errno 326", http.StatusInternalServerError)
		return
	}

	// Storage limit
	// 0 disables the limit
	// >= 1 enforces a limit, in number of gigabytes stored
	if h.limitStorage > 0 {
		if uint64(info.CurrentBytes) >= (h.limitStorage * 1024 * 1024 * 1024) {
			h.Error(w, r, fmt.Sprintf("Storage limit reached (currently consuming %s)", humanize.Bytes(uint64(info.CurrentBytes))), "Storage limit reached. Please try again later.\n", 633, http.StatusForbidden)
			return
		}
	}

	//fmt.Printf("Uploading filename %s (%s) to bin %s\n", inputFilename, humanize.Bytes(inputBytes), bin.Id)

	fp, err := ioutil.TempFile(h.tmpdir, "filebin")
	// Defer removal of the tempfile to clean up partially uploaded files.
	defer os.Remove(fp.Name())
	defer fp.Close()
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Failed to create temporary upload file %s: %s", fp.Name(), err.Error()), "Storage error", 124, http.StatusInternalServerError)
		return
	}

	t1 := time.Now()

	nBytes, err := io.Copy(fp, r.Body)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Failed to write temporary upload file %s: %s", fp.Name(), err.Error()), "Storage error", 125, http.StatusInternalServerError)
		return
	}
	if uint64(nBytes) != inputBytes {
		h.Error(w, r, fmt.Sprintf("Rejecting upload for file %s to bin %s since we got %d bytes and should have received %d bytes", inputFilename, bin.Id, nBytes, inputBytes), "Content-length did not match the request body length", 126, http.StatusBadRequest)
		return
	}
	if nBytes == 0 {
		h.Error(w, r, "", "Empty file uploads are not allowed", 127, http.StatusBadRequest)
		return
	}
	fp.Seek(0, 0)

	t2 := time.Now()

	md5_checksum := md5.New()
	if _, err := io.Copy(md5_checksum, fp); err != nil {
		h.Error(w, r, fmt.Sprintf("Error during checksum: %s", err.Error()), "Processing error", 128, http.StatusInternalServerError)
		return
	}
	md5_checksum_string := fmt.Sprintf("%x", md5_checksum.Sum(nil))
	if inputMD5 != "" {
		if md5_checksum_string != inputMD5 {
			h.Error(w, r, fmt.Sprintf("Rejecting upload for file %s to bin %s due to wrong MD5 checksum (got %s and calculated %s)", inputFilename, bin.Id, inputMD5, md5_checksum_string), "MD5 checksum did not match", 129, http.StatusBadRequest)
			return
		}
	}
	fp.Seek(0, 0)

	sha256_checksum := sha256.New()
	if _, err := io.Copy(sha256_checksum, fp); err != nil {
		h.Error(w, r, fmt.Sprintf("Error during checksum: %s", err.Error()), "Processing error", 130, http.StatusInternalServerError)
		return
	}
	sha256_checksum_string := fmt.Sprintf("%x", sha256_checksum.Sum(nil))
	if inputSHA256 != "" {
		if sha256_checksum_string != inputSHA256 {
			h.Error(w, r, fmt.Sprintf("Rejecting upload for file %s to bin %s due to wrong SHA256 checksum (got %s and calculated %s)", inputFilename, bin.Id, inputSHA256, sha256_checksum_string), "SHA256 checksum did not match", 130, http.StatusBadRequest)
			return
		}
	}
	fp.Seek(0, 0)

	mime, err := mimetype.DetectReader(fp)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Unable to detect mime type on filename %s in bin %s: %s", inputFilename, inputBin, err.Error()), "Processing error", 131, http.StatusInternalServerError)
		return
	}
	fp.Seek(0, 0)

	t3 := time.Now()

	// Check if file exists
	file, found, err := h.dao.File().GetByName(bin.Id, inputFilename)
	if err != nil {
		fmt.Printf("Unable to load file %s in bin %s: %s\n", file.Filename, bin.Id, err.Error())
		http.Error(w, "Errno 106", http.StatusInternalServerError)
		return
	}

	if found {
		// Increment the update counter if the file exists.
		file.Updates = file.Updates + 1
	}

	dump, err := httputil.DumpRequest(r, false)
	if err != nil {
		h.Error(w, r, "Failed to dump request", "Parse error", 135, http.StatusInternalServerError)
		return
	}
	file.Trace = string(dump)

	// Extract client IP
	ip, err := extractIP(r.RemoteAddr)
	if err != nil {
		h.Error(w, r, "Failed to dump request", "Parse error", 136, http.StatusInternalServerError)
		return
	}
	file.IP = ip

	// Set values according to the new file
	file.Filename = inputFilename
	file.Bin = bin.Id

	// Reset the deleted status and timestamp in case the file was deleted
	// earlier
	file.DeletedAt.Scan(nil)

	file.Bytes = inputBytes
	file.Mime = mime.String()
	file.SHA256 = sha256_checksum_string
	file.MD5 = md5_checksum_string
	if err := h.dao.File().ValidateInput(&file); err != nil {
		fmt.Printf("Rejected upload of filename %s to bin %s due to failed input validation: %s\n", inputFilename, bin.Id, err.Error())
		http.Error(w, "Input validation failed", http.StatusBadRequest)
		return
	}

	var nonce []byte

	// Retry if the S3 upload fails
	retry_limit := 3
	retry_counter := 1

	for {
		fp.Seek(0, 0)
		nonce, err = h.s3.PutObject(file.Bin, file.Filename, fp, nBytes)
		if err == nil {
			// Completed successfully
			h.s3.SetTrace(false)
			break
		} else {
			// Completed with error
			if retry_counter >= retry_limit {
				// Give up after a few attempts
				fmt.Printf("Gave up uploading to S3 after %d/%d attempts: %s\n", retry_counter, retry_limit, err.Error())
				http.Error(w, "Failed to store the object in S3, please try again later", http.StatusInternalServerError)
				return
			}
			fmt.Printf("Failed attempt to upload to S3 (%d/%d): %s\n", retry_counter, retry_limit, err.Error())

			retry_counter = retry_counter + 1

			// Sleep a little before retrying
			time.Sleep(time.Duration(retry_counter) * time.Second)

			// Get some more debug data in case the retry also fails
			h.s3.SetTrace(true)
		}
	}
	t4 := time.Now()

	file.Nonce = nonce
	file.InStorage = true

	if found {
		if err := h.dao.File().Update(&file); err != nil {
			fmt.Printf("Unable to update filename %s (id %d) in bin %s: %s\n", file.Filename, file.Id, bin.Id, err.Error())
			http.Error(w, "Errno 107", http.StatusInternalServerError)
			return
		}
	} else {
		if err := h.dao.File().Insert(&file); err != nil {
			fmt.Printf("Unable to insert file %s in bin %s: %s\n", file.Filename, bin.Id, err.Error())
			http.Error(w, "Errno 108", http.StatusInternalServerError)
			return
		}
		// TODO: Execute new file created trigger
	}

	// Update bin to set the correct updated timestamp
	bin.ExpiredAt = time.Now().UTC().Add(h.expirationDuration)
	if err := h.dao.Bin().Update(&bin); err != nil {
		fmt.Printf("Unable to update bin %s: %s\n", bin.Id, err.Error())
		http.Error(w, "Errno 109", http.StatusInternalServerError)
		return
	}

	fmt.Printf("Uploaded filename %s (%s) to bin %s (db in %.3fs, buffered in %.3fs, checksum in %.3fs, stored in %.3fs, total %.3fs)\n", file.Filename, humanize.Bytes(file.Bytes), bin.Id, t1.Sub(t0).Seconds(), t2.Sub(t1).Seconds(), t3.Sub(t2).Seconds(), t4.Sub(t3).Seconds(), t4.Sub(t0).Seconds())

	type Data struct {
		Bin  ds.Bin  `json:"bin"`
		File ds.File `json:"file"`
	}
	var data Data
	data.Bin = bin
	data.File = file

	w.Header().Set("Content-Type", "application/json")
	out, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		fmt.Printf("Failed to parse json: %s\n", err.Error())
		http.Error(w, "Errno 201", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	io.WriteString(w, string(out))
}

func (h *HTTP) DeleteFile(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	inputBin := params["bin"]
	inputFilename := params["filename"]

	bin, found, err := h.dao.Bin().GetById(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetById(%s): %s\n", inputBin, err.Error())
		http.Error(w, "Errno 110", http.StatusInternalServerError)
		return
	}
	if found == false {
		http.Error(w, "The file does not exist", http.StatusNotFound)
		return
	}

	if bin.IsReadable() == false {
		h.Error(w, r, "", "The bin is no longer available", 122, http.StatusNotFound)
		return
	}

	file, found, err := h.dao.File().GetByName(inputBin, inputFilename)
	if err != nil {
		fmt.Printf("Unable to GetByName(%s, %s): %s\n", inputBin, inputFilename, err.Error())
		http.Error(w, "Errno 111", http.StatusInternalServerError)
		return
	}
	if found == false {
		http.Error(w, "The file does not exist", http.StatusNotFound)
		return
	}

	// No need to delete the file twice
	if file.IsReadable() == false {
		http.Error(w, "This file is no longer available", http.StatusNotFound)
		return
	}

	// Flag as deleted
	now := time.Now().UTC().Truncate(time.Microsecond)
	file.DeletedAt.Scan(now)

	if err := h.dao.File().Update(&file); err != nil {
		fmt.Printf("Unable to update the file (%s, %s): %s\n", inputBin, inputFilename, err.Error())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Update the updated timestamp of the bin
	if err := h.dao.Bin().Update(&bin); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Error(w, "File deleted successfully", http.StatusOK)
	return
}
