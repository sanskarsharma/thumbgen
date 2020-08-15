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

func downloadFile(url string) *os.File {

	resp, err := http.Get(url)
	checkErr(err)
	defer resp.Body.Close()

	fmt.Println("Response status:", resp.Status)
	fmt.Println("response headers are: ", resp.Header.Get("content-type"))

	contentType := resp.Header.Get("content-type")
	contentTypeSplit := strings.Split(contentType, "/")
	extension := contentTypeSplit[len(contentTypeSplit) - 1]
	fmt.Println("extension = ", extension)

	// '*' will be populated with a random numeric string
	tempFile , err := ioutil.TempFile("", "download*." + extension)
	checkErr(err)
	respBody, err := ioutil.ReadAll(resp.Body)
	checkErr(err)
	tempFile.Write(respBody)

	fmt.Println("Temp file name:", tempFile.Name())
	return tempFile
}

func isImage(name string) bool {
	if strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg")|| strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".gif") {
		return true
	}
	return false
}

func generateImageThumbnail(file *os.File) *os.File  {
	sourcImg, err := imaging.Open(file.Name())
	checkErr(err)

	thumbnail := imaging.Thumbnail(sourcImg, 100, 100, imaging.CatmullRom)

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

func handleThumbify(w http.ResponseWriter, req *http.Request)  {

	// Read body
	body, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Unmarshal into request payload struct
	var thumbnailRequestPayload ThumbnailRequestPayload
	err = json.Unmarshal(body, &thumbnailRequestPayload)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// download file
	file := downloadFile(thumbnailRequestPayload.DownloadUrl)
	// defer os.Remove(file.Name())
	fmt.Println(file.Name())

	// call thumbgen for generating thumbnail
	var outputFile *os.File 
	if isImage(file.Name()) {
		outputFile = generateImageThumbnail(file)
	}
	fmt.Println(outputFile.Name())

	// upload file 
	// todo : raise error if upload response non 200
	uploadFile(outputFile, thumbnailRequestPayload.UploadUrl)

	// write response and return
	// fmt.Fprintf(w, output_filepath)
}


type ThumbnailRequestPayload struct {
	DownloadUrl string `json:"download_url"`
	UploadUrl string `json:"upload_url"`
}


func RecoveryWrapper(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			err := recover()
			if err != nil {
				fmt.Println(err) // May be log this error? Send to sentry?

				jsonBody, _ := json.Marshal(map[string]string{
					"error": "There was an internal server error",
				})
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write(jsonBody)
			}

		}()
		handler.ServeHTTP(w, r)
	})
}


func main()  {
	m := mux.NewRouter()
    m.Handle("/thumbify", RecoveryWrapper(http.HandlerFunc(handleThumbify))).Methods("POST")

    http.Handle("/", m)
	log.Println("Listening...")
	http.ListenAndServe(":2712", nil)
}

