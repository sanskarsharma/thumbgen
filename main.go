package main

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"os"
	"strings"
	"image"
	"image/color"
    "encoding/json"
	"bytes"
	"log"

	"github.com/disintegration/imaging"
	"github.com/gorilla/mux"
)

func checkErr(e error) {
    if e != nil {
        panic(e)
    }
}

func isVideo(contentType string) bool {
	contentTypeSplit := strings.Split(contentType, "/")
	extension := contentTypeSplit[len(contentTypeSplit) - 1]
	log.Println("extension = ", extension)

	for _, ext := range []string{"mp4", "3gp", "mpv", "x-flv", "mov", "quicktime", "raw", "x-msvideo", "x-ms-wmv"} {
		if extension == ext {
			return true
		}
	}
	return false
}

func isImage(contentType string) bool {
	contentTypeSplit := strings.Split(contentType, "/")
	extension := contentTypeSplit[len(contentTypeSplit) - 1]
	log.Println("extension = ", extension)

	for _, ext := range []string{"jpeg", "jpg", "png", "gif"} {
		if extension == ext {
			return true
		}
	}
	return false
}

func generateImageThumbnail(file *os.File) *os.File  {
	sourcImg, err := imaging.Open(file.Name())
	checkErr(err)

	thumbnail := imaging.Thumbnail(sourcImg, 200, 200, imaging.CatmullRom)

	// create a new blank image
	thumbnailImg := imaging.New(200, 200, color.NRGBA{0, 0, 0, 0})

	// paste thumbnails into the new image
	thumbnailImg = imaging.Paste(thumbnailImg, thumbnail, image.Pt(0, 0))

	// save the combined image to file
	tempFile , err := ioutil.TempFile("", "thumbnail*.png")
	checkErr(err)
	err = imaging.Save(thumbnailImg, tempFile.Name())
	checkErr(err)

	return tempFile
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
	fmt.Println(string(respBody))

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Non-OK HTTP status:", resp.StatusCode)
		panic("uploading thumbnail failed") 
	}

    defer resp.Body.Close()
}

func handleThumbify(w http.ResponseWriter, r *http.Request)  {

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
	if (thumbnailRequestPayload.DownloadUrl == "" || thumbnailRequestPayload.UploadUrl == "") {
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
		extension := contentTypeSplit[len(contentTypeSplit) - 1]

		tempFile , err := ioutil.TempFile("", "download*." + extension) // '*' will be populated with a random numeric string
		checkErr(err)
		defer os.Remove(tempFile.Name())

		respBody, err := ioutil.ReadAll(downloadResponse.Body)
		checkErr(err)

		tempFile.Write(respBody)
		log.Println("temp file name:", tempFile.Name())

		outputFile = generateImageThumbnail(tempFile)

	} else if isVideo(contentType) {
		// TODO 
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
	UploadUrl string `json:"upload_url"`
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

func contentTypeMiddleware(handler http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Add("Content-Type", "application/json")
        handler.ServeHTTP(w, r)
    })
}

func main()  {
	router := mux.NewRouter()
	router.Use(contentTypeMiddleware)
	router.Use(recoveryMiddleware)

    router.HandleFunc("/thumbify", handleThumbify).Methods("POST")

	log.Println("Listening...")
	http.ListenAndServe(":2712", router)
}

