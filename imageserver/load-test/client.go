// Copyright 2020 Google LLC
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

// Binary load-test sends synthetic requests to the image server.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	imagePath    = flag.String("images_path", "data", "location of test images grouped by tag")
	targetHost   = flag.String("target_host", "127.0.0.1", "IP address or hostname to test")
	uploadRate   = flag.Int("upload_rate", 1, "request rate of upload requests")
	uiRate       = flag.Int("ui_rate", 1, "request rate of ui requests")
	searchRate   = flag.Int("search_rate", 1, "request rate of search requests")
	downloadRate = flag.Int("download_rate", 1, "request rate of download requests")
	testDuration = flag.Duration("test_duration", 10*time.Minute, "duration of the load test")
	rampupTime   = flag.Duration("rampup_time", 2*time.Minute, "duration of the rampup to test rate")
	userCount    = flag.Int("user_count", 1000, "number of random users to generate")
	workers      = flag.Int("workers", 200, "number of parallel workers for the test")
)

type image struct {
	path string
	tag  string
}

type report struct {
	time time.Duration
	err  error
}

type testClient interface {
	makeRequest() error
}

type uploadClient struct {
	host   string
	images []image
}

type searchClient struct {
	host         string
	tags         []string
	downloadChan chan string
}

type downloadClient struct {
	host         string
	downloadChan chan string
}

type uiClient struct {
	host string
}

func (c *uploadClient) makeRequest() error {
	url := "http://" + c.host + "/upload"
	buf := &bytes.Buffer{}
	upload := multipart.NewWriter(buf)
	i := rand.Intn(len(c.images))
	path := c.images[i].path
	tag := c.images[i].tag

	err := upload.WriteField("username", generateUser())
	if err != nil {
		return err
	}

	tagsJson, err := json.Marshal([]string{tag})
	if err != nil {
		return err
	}
	err = upload.WriteField("hashtags", string(tagsJson))
	if err != nil {
		return err
	}

	writer, err := upload.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	if err != nil {
		return err
	}

	upload.Close()
	contentType := upload.FormDataContentType()
	resp, err := http.Post(url, contentType, buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil

}

func (c *uiClient) makeRequest() error {
	resp, err := http.Get("http://" + c.host + "/")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	if err != nil {
		return err
	}
	want := "UiFrontend"
	if !bytes.Contains(body, []byte(want)) {
		return fmt.Errorf("page content does not match: %s", want)
	}
	return nil
}

func (c *searchClient) makeRequest() error {
	i := rand.Intn(len(c.tags))
	tag := c.tags[i]

	resp, err := http.PostForm("http://"+c.host+"/search",
		url.Values{"keyword": {tag}})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	if err != nil {
		return err
	}
	var thumbnails []string
	err = json.Unmarshal(body, &thumbnails)
	if err != nil {
		return err
	}

LOOP:
	for _, thumb := range thumbnails {
		select {
		case c.downloadChan <- thumb:
		default:
			break LOOP
		}
	}
	return nil
}

func (c *downloadClient) makeRequest() error {
	var url string

	select {
	case url = <-c.downloadChan:
	default:
	}

	if url == "" {
		return fmt.Errorf("no download urls found")
	}

	select {
	case c.downloadChan <- url:
	default:
	}

	thumbPrefix := "/download/thumbnail_"
	if rand.Intn(100) == 0 && strings.HasPrefix(url, thumbPrefix) {
		url = "/download/" + strings.TrimPrefix(url, thumbPrefix)
	}

	resp, err := http.Get("http://" + c.host + url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return fmt.Errorf("download returned empty body")
	}
	return nil
}

func loadImages(baseDir string) ([]image, error) {
	images := make([]image, 0)
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".png" {
			return nil
		}
		tag := filepath.Base(filepath.Dir(path))
		images = append(images, image{path, tag})
		return nil
	})
	if len(images) == 0 && err == nil {
		err = fmt.Errorf("no image files found")
	}
	return images, err
}

func checkHost(host string) error {
	client := uiClient{host}
	return client.makeRequest()
}

func generateUser() string {
	return fmt.Sprintf("user%d", rand.Intn(*userCount)+1)
}

func collectTags(images []image) []string {
	tags := []string{""}
	seen := make(map[string]bool)
	for _, img := range images {
		if seen[img.tag] {
			continue
		}
		seen[img.tag] = true
		tags = append(tags, img.tag)
	}
	return tags
}

func reporter(name string, reportChan chan report, done chan struct{}) {
	start := time.Now()
	tick := 10 * time.Second
	ticker := time.NewTicker(tick)
	reports := make([]report, 0)

LOOP:
	for {
		select {
		case <-done:
			break LOOP
		case r := <-reportChan:
			reports = append(reports, r)
		case <-ticker.C:
			logReports(name, time.Since(start), reports)
			reports = reports[:0]
			start = time.Now()
		}
	}

	ticker.Stop()
}

func logReports(name string, tick time.Duration, reports []report) {
	var totalTime int64
	errors := make(map[string]int)
	totalErrors := 0
	for _, r := range reports {
		totalTime += r.time.Nanoseconds() / 1000000
		if r.err != nil {
			errors[r.err.Error()]++
			totalErrors++
		}
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].time < reports[j].time })

	qps := float64(len(reports)) / tick.Seconds()
	msg := fmt.Sprintf("%s: %0.1f req/s", name, qps)
	if len(reports) > 0 {
		avg := totalTime / int64(len(reports))
		p99 := len(reports) * 99 / 100
		msg += fmt.Sprintf(", avg %dms, p99 %dms, %d errors",
			avg, reports[p99].time.Nanoseconds()/1000000, totalErrors)
	}
	log.Println(msg)
	for err, n := range errors {
		if n == 1 {
			log.Printf("%s: %s\n", name, err)
		} else {
			log.Printf("%s: %s (%d times)\n", name, err, n)
		}
	}
}

func loadTest(name string, client testClient, rate int, rampup time.Duration, workers int, done chan struct{}) {
	log.Printf("%s: starting load test with %d requests per second\n", name, rate)
	ticker := time.NewTicker(time.Second / time.Duration(rate))
	rampupStart := time.Now()
	rampupEnd := rampupStart.Add(rampup)
	rampupDone := rampup == 0
	reportChan := make(chan report)
	go reporter(name, reportChan, done)

	requestChan := make(chan struct{})
	for i := 0; i < workers; i++ {
		go func() {
			for {
				select {
				case <-done:
					return
				case <-requestChan:
					start := time.Now()
					err := client.makeRequest()
					reportChan <- report{time.Since(start), err}
				}
			}
		}()
	}

LOOP:
	for {
		select {
		case <-done:
			break LOOP
		case <-ticker.C:
		}

		if !rampupDone {
			now := time.Now()
			if now.After(rampupEnd) {
				rampupDone = true
				log.Printf("%s: rampup done\n", name)
			} else {
				percent := int(now.Sub(rampupStart) * 100 / rampup)
				if rand.Intn(100) > percent {
					continue
				}
			}
		}

		select {
		case requestChan <- struct{}{}:
		default:
		}
	}

	ticker.Stop()
}

func main() {
	flag.Parse()

	images, err := loadImages(*imagePath)
	if err != nil {
		log.Fatalf("failed to load images from path '%s': %v", *imagePath, err)
	}
	log.Printf("loaded %d images from: %s\n", len(images), *imagePath)

	err = checkHost(*targetHost)
	if err != nil {
		log.Fatalf("failed to check host %s: %v", *targetHost, err)
	}
	log.Println("target host is alive:", *targetHost)

	done := make(chan struct{})
	downloadChan := make(chan string, 1000)
	if *uploadRate > 0 {
		client := &uploadClient{*targetHost, images}
		go loadTest("upload", client, *uploadRate, *rampupTime, *workers, done)
	}
	if *uiRate > 0 {
		client := &uiClient{*targetHost}
		go loadTest("ui", client, *uiRate, *rampupTime, *workers, done)
	}
	if *searchRate > 0 {
		client := &searchClient{*targetHost, collectTags(images), downloadChan}
		go loadTest("search", client, *searchRate, *rampupTime, *workers, done)
	}
	if *downloadRate > 0 {
		client := &downloadClient{*targetHost, downloadChan}
		go loadTest("download", client, *downloadRate, *rampupTime, *workers, done)
	}

	time.Sleep(*rampupTime + *testDuration)
	close(done)
}
