// Copyright 2013 SourceGraph, Inc.
// Copyright 2011-2013 Numrotron Inc.
// Use of this source code is governed by an MIT-style license
// that can be found in the LICENSE file.
package ses

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config specifies configuration options and credentials for accessing Amazon SES.
type Config struct {
	// AccessKeyID is your Amazon AWS access key ID.
	AccessKeyID string

	// SecretAccessKey is your Amazon AWS secret key.
	SecretAccessKey string

	// Endpoint
	Endpoint string
}

type GetSendQuotaResult struct {
	SentLast24Hours float64
	Max24HourSend   float64
	MaxSendRate     float64
}

type GetSendQuotaResponse struct {
	GetSendQuotaResult GetSendQuotaResult
}

type SendDataPoint struct {
	Complaints       int
	DeliveryAttempts int
	Bounces          int
	Rejects          int
	Timestamp        time.Time
}

type GetSendStatisticsResult struct {
	SendDataPoints []SendDataPoint `xml:"SendDataPoints>member"`
}

type GetSendStatisticsResponse struct {
	GetSendStatisticsResult GetSendStatisticsResult
}

func (c *Config) SendEmail(from, to, subject, body string) (string, error) {
	data := make(url.Values)
	data.Add("Action", "SendEmail")
	data.Add("Source", from)
	data.Add("Destination.ToAddresses.member.1", to)
	data.Add("Message.Subject.Data", subject)
	data.Add("Message.Body.Text.Data", body)
	data.Add("AWSAccessKeyId", c.AccessKeyID)

	return sesPost(data, c.AccessKeyID, c.SecretAccessKey, c.Endpoint)
}

func (c *Config) SendEmailHTML(from, to, subject, bodyText, bodyHTML string) (string, error) {
	data := make(url.Values)
	data.Add("Action", "SendEmail")
	data.Add("Source", from)
	data.Add("Destination.ToAddresses.member.1", to)
	data.Add("Message.Subject.Data", subject)
	data.Add("Message.Body.Text.Data", bodyText)
	data.Add("Message.Body.Html.Data", bodyHTML)
	data.Add("AWSAccessKeyId", c.AccessKeyID)

	return sesPost(data, c.AccessKeyID, c.SecretAccessKey, c.Endpoint)
}

func (c *Config) SendRawEmail(raw []byte) (string, error) {
	data := make(url.Values)
	data.Add("Action", "SendRawEmail")
	data.Add("RawMessage.Data", base64.StdEncoding.EncodeToString(raw))
	data.Add("AWSAccessKeyId", c.AccessKeyID)

	return sesPost(data, c.AccessKeyID, c.SecretAccessKey, c.Endpoint)
}

func (c *Config) GetSendQuota() (GetSendQuotaResult, error) {
	data := make(url.Values)
	data.Add("Action", "GetSendQuota")
	data.Add("AWSAccessKeyId", c.AccessKeyID)

	body, err := sesGet(data, c.AccessKeyID, c.SecretAccessKey, c.Endpoint)
	if err != nil {
		return GetSendQuotaResult{}, err
	}

	res := GetSendQuotaResponse{}
	err = xml.Unmarshal([]byte(body), &res)
	return res.GetSendQuotaResult, err
}

func (c *Config) GetSendStatistics() ([]SendDataPoint, error) {
	data := make(url.Values)
	data.Add("Action", "GetSendStatistics")
	data.Add("AWSAccessKeyId", c.AccessKeyID)

	body, err := sesGet(data, c.AccessKeyID, c.SecretAccessKey, c.Endpoint)
	if err != nil {
		return []SendDataPoint{}, err
	}

	res := GetSendStatisticsResponse{}

	err = xml.Unmarshal([]byte(body), &res)

	return res.GetSendStatisticsResult.SendDataPoints, err
}

func authorizationHeader(date, accessKeyID, secretAccessKey string) []string {
	h := hmac.New(sha256.New, []uint8(secretAccessKey))
	h.Write([]uint8(date))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	auth := fmt.Sprintf("AWS3-HTTPS AWSAccessKeyId=%s, Algorithm=HmacSHA256, Signature=%s", accessKeyID, signature)
	return []string{auth}
}

func sesGet(data url.Values, accessKeyID, secretAccessKey, endpoint string) (string, error) {
	urlstr := fmt.Sprintf("%s?%s", endpoint, data.Encode())
	endpointURL, _ := url.Parse(urlstr)
	headers := map[string][]string{}

	now := time.Now().UTC()
	// date format: "Tue, 25 May 2010 21:20:27 +0000"
	date := now.Format("Mon, 02 Jan 2006 15:04:05 -0700")
	headers["Date"] = []string{date}

	h := hmac.New(sha256.New, []uint8(secretAccessKey))
	h.Write([]uint8(date))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	auth := fmt.Sprintf("AWS3-HTTPS AWSAccessKeyId=%s, Algorithm=HmacSHA256, Signature=%s", accessKeyID, signature)
	headers["X-Amzn-Authorization"] = []string{auth}

	req := http.Request{
		URL:        endpointURL,
		Method:     "GET",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Close:      true,
		Header:     headers,
	}

	r, err := http.DefaultClient.Do(&req)
	if err != nil {
		log.Printf("http error: %s", err)
		return "", err
	}

	resultbody, _ := ioutil.ReadAll(r.Body)
	r.Body.Close()

	if r.StatusCode != 200 {
		log.Printf("error, status = %d", r.StatusCode)

		log.Printf("error response: %s", resultbody)
		return "", errors.New(string(resultbody))
	}

	return string(resultbody), nil
}

func sesPost(data url.Values, accessKeyID, secretAccessKey, endpoint string) (string, error) {
	body := strings.NewReader(data.Encode())
	req, err := http.NewRequest("POST", endpoint, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	now := time.Now().UTC()
	// date format: "Tue, 25 May 2010 21:20:27 +0000"
	date := now.Format("Mon, 02 Jan 2006 15:04:05 -0700")
	req.Header.Set("Date", date)

	h := hmac.New(sha256.New, []uint8(secretAccessKey))
	h.Write([]uint8(date))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	auth := fmt.Sprintf("AWS3-HTTPS AWSAccessKeyId=%s, Algorithm=HmacSHA256, Signature=%s", accessKeyID, signature)
	req.Header.Set("X-Amzn-Authorization", auth)

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("http error: %s", err)
		return "", err
	}

	resultbody, _ := ioutil.ReadAll(r.Body)
	r.Body.Close()

	if r.StatusCode != 200 {
		log.Printf("error, status = %d", r.StatusCode)

		log.Printf("error response: %s", resultbody)
		return "", errors.New(fmt.Sprintf("error code %d. response: %s", r.StatusCode, resultbody))
	}

	return string(resultbody), nil
}
