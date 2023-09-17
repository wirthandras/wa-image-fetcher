package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type DownloadImageRequest struct {
	ExternalImageUrl string `json:"externalImageUrl"`
}

func fetchImage(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var data DownloadImageRequest

	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, data.ExternalImageUrl)

	uuidWithHyphen := uuid.New()

	filename := fmt.Sprintf("%s.jpg", uuidWithHyphen)

	downloadImage(data.ExternalImageUrl, filename)

	uploadImageToMinio(filename)

	callback(data.ExternalImageUrl, filename)

	removeFile(filename)
}

func handleRequests() {
	http.HandleFunc("/", fetchImage)
	port := os.Getenv("WA_CRAWLERS_IMAGE_DOWNLOADER_PORT")
	addr := fmt.Sprintf(":%s", port)

	log.Fatal(http.ListenAndServe(addr, nil))
}

func main() {
	handleRequests()
}

func downloadImage(imageURL string, filename string) {

	resp, err := http.Get(imageURL)
	if err != nil {
		fmt.Println("Error sending GET request:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Received non-success status code:", resp.Status)
		return
	}

	file, err := os.Create(filename)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		fmt.Println("Error copying response body to file:", err)
		return
	}

	fmt.Println("Image downloaded successfully.")
}

func uploadImageToMinio(filename string) {
	ctx := context.Background()
	endpoint := os.Getenv("WA_CRAWLERS_MINIO_URL")
	accessKeyID := os.Getenv("WA_CRAWLERS_MINIO_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("WA_CRAWLERS_MINIO_SECRET_ACCESS_KEY")
	bucketName := os.Getenv("WA_CRAWLERS_IMAGE_DOWNLOADER_MINIO_BUCKET_NAME")
	useSSL := false

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln(err)
	}

	objectName := filename
	filePath := filename
	contentType := "image/jpg"

	info, err := minioClient.FPutObject(ctx, bucketName, objectName, filePath, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Successfully uploaded %s of size %d\n", objectName, info.Size)
}

func removeFile(filename string) {
	e := os.Remove(filename)
	if e != nil {
		log.Fatal(e)
	}
}

type ImagePutRequest struct {
	ExternalImageUrl string `json:"externalImageUrl"`
	Url              string `json:"internalImageUrl"`
}

func callback(externalImageUrl string, filename string) {

	endpoint := os.Getenv("WA_CRAWLERS_MINIO_URL")
	bucketName := os.Getenv("WA_CRAWLERS_IMAGE_DOWNLOADER_MINIO_BUCKET_NAME")
	imageUrl := fmt.Sprintf("http://%s/%s/%s", endpoint, bucketName, filename)

	data, _ := json.Marshal(map[string]string{
		"externalImageUrl": externalImageUrl,
		"internalImageUrl": imageUrl,
	})

	requestBody := bytes.NewBuffer(data)

	url := os.Getenv("WA_CRAWLERS_IMAGE_CALLBACK_RESOURCE")
	request, err := http.NewRequest("PUT", url, requestBody)
	if err != nil {
		fmt.Println("Error PUT Request creation:", err)
		return
	}

	request.Header.Add("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		fmt.Println("Error at PUT callback:", err)
		return
	}
	defer response.Body.Close()
}
