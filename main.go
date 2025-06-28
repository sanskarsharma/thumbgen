package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/gorilla/mux"
)

func checkErr(e error) {
	if e != nil {
		panic(e)
	}
}

func isVideo(contentType string) bool {

	supportedContentTypes := []string{"video/mp4", "video/3gpp", "video/mpv", "video/x-flv", "video/quicktime", "video/quicktime", "video/raw", "video/x-msvideo", "video/x-ms-wmv", "video/webm"} // default
	jsonString := os.Getenv("SUPPORTED_VIDEO_CONTENT_TYPES")
	if jsonString != "" {
		err := json.Unmarshal([]byte(jsonString), &supportedContentTypes)
		checkErr(err)
	}

	for _, supportedContentType := range supportedContentTypes {
		if contentType == supportedContentType {
			return true
		}
	}
	return false
}

func isImage(contentType string) bool {

	supportedContentTypes := []string{"image/jpeg", "image/jpeg", "image/png", "image/gif", "image/bmp", "image/svg+xml", "image/tiff"} // default
	jsonString := os.Getenv("SUPPORTED_IMAGE_CONTENT_TYPES")
	if jsonString != "" {
		err := json.Unmarshal([]byte(jsonString), &supportedContentTypes)
		checkErr(err)
	}

	for _, supportedContentType := range supportedContentTypes {
		if contentType == supportedContentType {
			return true
		}
	}
	return false
}

func generateImageThumbnail(file *os.File) *os.File {
	sourcImg, err := imaging.Open(file.Name())
	checkErr(err)

	thumbnail := imaging.Thumbnail(sourcImg, 200, 200, imaging.CatmullRom)

	// create a new blank image
	thumbnailImg := imaging.New(200, 200, color.NRGBA{0, 0, 0, 0})

	// paste thumbnails into the new image
	thumbnailImg = imaging.Paste(thumbnailImg, thumbnail, image.Pt(0, 0))

	// save the combined image to file
	tempFile, err := ioutil.TempFile("", "thumbnail*.png")
	checkErr(err)
	err = imaging.Save(thumbnailImg, tempFile.Name())
	checkErr(err)

	return tempFile
}

func generateVideoThumbnail(url string) *os.File {

	tempDir, err := ioutil.TempDir("", "thumbnail*")
	checkErr(err)

	outputFilePath := tempDir + "/thumbnail.png"

	cmd := `ffmpeg -i "%s" -an -q 0 -vf scale="'if(gt(iw,ih),-1,200):if(gt(iw,ih),200,-1)', crop=200:200:exact=1" -vframes 1 "%s"`
	// ffmpeg cmd ref : https://gist.github.com/TimothyRHuertas/b22e1a252447ab97aa0f8de7c65f96b8

	cmdSubstituted := fmt.Sprintf(cmd, url, outputFilePath)

	shellName := "ash" // for docker (using alpine image)
	if os.Getenv("ENV") != "" && os.Getenv("ENV") == "LOCAL" {
		shellName = "bash"
	}

	ffCmd := exec.Command(shellName, "-c", cmdSubstituted)

	// getting real error msg : https://stackoverflow.com/questions/18159704/how-to-debug-exit-status-1-error-when-running-exec-command-in-golang
	output, err := ffCmd.CombinedOutput()
	if err != nil {
		log.Println(fmt.Sprint(err) + ": " + string(output))
		checkErr(err)
	}
	log.Println(string(output))

	outputFile, err := os.Open(outputFilePath)
	return outputFile
}

func uploadFile(file *os.File, uploadUrl string) {

	fileBytes, err := ioutil.ReadFile(file.Name())
	checkErr(err)

	fmt.Println("uploading to ", uploadUrl)
	uploadRequest, err := http.NewRequest("PUT", uploadUrl, bytes.NewBuffer(fileBytes))
	checkErr(err)
	httpClient := &http.Client{}
	resp, err := httpClient.Do(uploadRequest)
	checkErr(err)

	fmt.Println(resp.Status)
	fmt.Println(resp.Body)
	respBody, err := ioutil.ReadAll(resp.Body)
	checkErr(err)
	defer resp.Body.Close()
	fmt.Println(string(respBody))

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Non-OK HTTP status:", resp.StatusCode)
		panic("uploading thumbnail failed")
	}

}

func handleThumbify(w http.ResponseWriter, r *http.Request) {

	// Read body
	body, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// Unmarshal into request payload struct
	var thumbnailRequestPayload ThumbnailRequestPayload
	err = json.Unmarshal(body, &thumbnailRequestPayload)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// check if both requried fields are present
	if thumbnailRequestPayload.DownloadUrl == "" || thumbnailRequestPayload.UploadUrl == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"message": "download_url or upload_url key not present in request data"})
		return
	}

	// real work starts from here

	// make GET request with "Range" header to get partial content in response and check response header for content type
	// this ^ is valid for s3 URLs only and is a workaround since HEAD requests cannot be made to s3 presigned URLs (ref : https://stackoverflow.com/a/39663152/7314323)

	// make GET request and check content-type
	downloadResponse, err := http.Get(thumbnailRequestPayload.DownloadUrl)
	checkErr(err)
	defer downloadResponse.Body.Close()
	if !(downloadResponse.StatusCode >= 200 && downloadResponse.StatusCode <= 299) {
		// raise 422
		w.WriteHeader(422)
		jsonResponse, _ := json.Marshal(map[string]string{
			"message": fmt.Sprintf("download url returned %d status code", downloadResponse.StatusCode),
		})
		w.Write(jsonResponse)
		return
	}

	log.Println("Response status:", downloadResponse.Status)
	log.Println("response headers are: ", downloadResponse.Header.Get("content-type"))

	contentType := downloadResponse.Header.Get("content-type")

	var outputFile *os.File
	if isImage(contentType) {
		contentTypeSplit := strings.Split(contentType, "/")
		extension := contentTypeSplit[len(contentTypeSplit)-1]

		tempFile, err := ioutil.TempFile("", "download*."+extension) // '*' will be populated with a random numeric string
		checkErr(err)
		defer os.Remove(tempFile.Name())
		log.Println("temp file name:", tempFile.Name())

		// reading response body from the GET call and directly writing it to file without keeping in memory. ref : https://stackoverflow.com/a/11693049/7314323
		_, err = io.Copy(tempFile, downloadResponse.Body)
		checkErr(err)

		outputFile = generateImageThumbnail(tempFile)

	} else if isVideo(contentType) {
		outputFile = generateVideoThumbnail(thumbnailRequestPayload.DownloadUrl)
	} else {
		// raise 422
		w.WriteHeader(422)
		jsonResponse, _ := json.Marshal(map[string]string{
			"message": "Un-supported content type",
		})
		w.Write(jsonResponse)
		return
	}
	log.Println("thumbnail file :", outputFile.Name())

	// upload file
	// todo : raise error if upload response non 200
	uploadFile(outputFile, thumbnailRequestPayload.UploadUrl)

}

type ThumbnailRequestPayload struct {
	DownloadUrl string `json:"download_url"`
	UploadUrl   string `json:"upload_url"`
}

func recoveryMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			err := recover()
			if err != nil {
				fmt.Println(err) // May be log this error? Send to sentry?

				jsonBody, _ := json.Marshal(map[string]string{
					"error": "There was an internal server error",
				})
				w.WriteHeader(http.StatusInternalServerError)
				w.Write(jsonBody)
			}

		}()
		handler.ServeHTTP(w, r)
	})
}

func main() {
	router := mux.NewRouter()

	// Apply recovery middleware to all routes
	router.Use(recoveryMiddleware)

	// Serve the frontend on the root route
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFile(w, r, "public/index.html")
	}).Methods("GET")

	// Serve static files from public directory
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("public/"))))

	router.HandleFunc("/thumbify", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		handleThumbify(w, r)
	}).Methods("POST")

	log.Println("Listening on :4499...")
	log.Println("Frontend available at: http://localhost:4499")
	log.Println("API endpoint: http://localhost:4499/thumbify")
	http.ListenAndServe(":4499", router)
}
