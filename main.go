package main

import (
	"bytes"
	"common/constant"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
	"github.com/zeebo/errs"
	"io/ioutil"
	"net/http"
	"net/url"
	console "storj.io/storj/satellite/console/consolewasm"
	"strconv"
	"strings"
	"time"
)

func main() {
	if err := InitTrans("zh"); err != nil {
		fmt.Printf("init trans failed, err:%v\n", err)
	}
	r := gin.Default()
	r.POST("/createCredentials", createCredentials)
	r.POST("/createRestrictKey", createRestrictKey)
	r.POST("/createAccessGrant", createAccessGrant)
	r.POST("/createBucket", createBucket)
	r.DELETE("/deleteBucket", deleteBucket)
	r.POST("/headBucket", headBucket)
	r.POST("/listBuckets", listBuckets)
	r.POST("/listObjects", listObjects)
	r.POST("/copyObject", copyObject)
	r.DELETE("/deleteObject", deleteObject)
	r.DELETE("/deleteObjects", deleteObjects)
	r.POST("/moveObject", moveObject)
	r.POST("/uploadObject", uploadObject)
	r.POST("/downloadObject", downloadObject)
	r.POST("/headObject", headObject)
	r.POST("/putObjectTagging", putObjectTagging)
	r.POST("/getObjectTagging", getObjectTagging)
	r.DELETE("/deleteObjectTagging", deleteObjectTagging)
	r.POST("/createMultipartUpload", createMultipartUpload)
	r.POST("/abortMultipartUpload", abortMultipartUpload)
	r.POST("/completeMultipartUpload", completeMultipartUpload)
	r.POST("/listMultipartUploads", listMultipartUploads)
	r.POST("/listParts", listParts)
	r.POST("/uploadPart", uploadPart)
	r.POST("/shareUrl", shareUrl)
	//r.POST("/createCredentialsByAccount", createCredentialsByAccount)
	r.POST("/getAccessGrantByAccount", getAccessGrantByAccount)

	r.Run()
}

func createCredentials(c *gin.Context) {
	type CreateCredentialsRequest struct {
		AccessGrant string `form:"access_grant" json:"access_grant" binding:"required"`
		AuthService string `form:"auth_service" json:"auth_service" binding:"required"`
	}
	var request CreateCredentialsRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	var cs, err = getCredentials(request.AuthService, request.AccessGrant, true)
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		responseForPostForm(c, constant.Success, "success", cs)
	}
}

func createBucket(c *gin.Context) {
	type CreateBucketRequest struct {
		Bucket string `form:"bucket" json:"bucket" binding:"required"`
	}
	var request CreateBucketRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	client := createS3Client(c)
	_, err := client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket:                    aws.String(request.Bucket),
		ACL:                       types.BucketCannedACLPrivate,
		CreateBucketConfiguration: &types.CreateBucketConfiguration{LocationConstraint: types.BucketLocationConstraintUsWest2},
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		responseForPostForm(c, constant.Success, "创建成功", nil)
	}
}

func deleteBucket(c *gin.Context) {
	type DeleteBucketRequest struct {
		Bucket string `form:"bucket" json:"bucket" binding:"required"`
	}
	var request DeleteBucketRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	client := createS3Client(c)
	_, err := client.DeleteBucket(context.TODO(), &s3.DeleteBucketInput{
		Bucket: aws.String(request.Bucket),
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		responseForPostForm(c, constant.Success, "删除成功", nil)
	}
}

func headBucket(c *gin.Context) {
	type HeadBucketRequest struct {
		Bucket string `form:"bucket" json:"bucket" binding:"required"`
	}
	var request HeadBucketRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	client := createS3Client(c)
	_, err := client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(request.Bucket),
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
	} else {
		responseForPostForm(c, constant.Success, "success", nil)
	}
}

func listBuckets(c *gin.Context) {
	client := createS3Client(c)
	listBucketsResult, err := client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		type buc struct {
			Bucket  string `json:"bucket"`
			Created int64  `json:"created"`
		}
		var buckets = make([]buc, 0)
		for _, bucket := range listBucketsResult.Buckets {
			var b buc
			b.Bucket = *bucket.Name
			b.Created = getFormatTime(bucket.CreationDate)
			buckets = append(buckets, b)
		}
		responseForPostForm(c, constant.Success, "success", buckets)
	}
}

func listObjects(c *gin.Context) {
	type ListObjectsRequest struct {
		Bucket            string `form:"bucket" json:"bucket" binding:"required"`
		Prefix            string `form:"prefix" json:"prefix"`
		Delimiter         string `form:"delimiter" json:"delimiter"`
		MaxKeys           string `form:"max_keys" json:"max_keys"`
		ContinuationToken string `form:"continuation_token" json:"continuation_token"`
	}
	var request ListObjectsRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	client := createS3Client(c)
	var maxKeys int
	var err error
	if len(request.MaxKeys) != 0 {
		maxKeys, err = strconv.Atoi(request.MaxKeys)
		if err != nil {
			responseForPostForm(c, constant.UnknownError, err.Error(), nil)
			return
		}
	}
	var continuationToken *string
	if len(request.ContinuationToken) != 0 {
		continuationToken = aws.String(request.ContinuationToken)
	}
	listObjsResponse, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket:            aws.String(request.Bucket),
		Prefix:            aws.String(request.Prefix),
		Delimiter:         aws.String(request.Delimiter),
		MaxKeys:           int32(maxKeys),
		ContinuationToken: continuationToken,
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		type obj struct {
			Key  string `json:"key"`
			Size int64  `json:"size"`
			Kind string `json:"kind"`
		}
		type Response struct {
			Files                 []obj  `json:"files"`
			IsTruncated           bool   `json:"is_truncated"`
			NextContinuationToken string `json:"next_continuation_token"`
		}
		var response Response
		var objects = make([]obj, 0)
		for _, pre := range listObjsResponse.CommonPrefixes {
			objects = append(objects, obj{
				Key:  *pre.Prefix,
				Kind: "PRE",
			})
		}
		for _, object := range listObjsResponse.Contents {
			objects = append(objects, obj{
				Key:  *object.Key,
				Kind: "OBJ",
				Size: object.Size,
			})
		}
		response.Files = objects
		response.IsTruncated = listObjsResponse.IsTruncated
		if listObjsResponse.NextContinuationToken != nil {
			response.NextContinuationToken = *listObjsResponse.NextContinuationToken
		}
		responseForPostForm(c, constant.Success, "success", response)
	}
}

func copyObject(c *gin.Context) {
	type CopyObjectRequest struct {
		SourceBucket      string `form:"source_bucket" json:"source_bucket" binding:"required"`
		SourceKey         string `form:"source_key" json:"source_key" binding:"required"`
		DestinationBucket string `form:"destination_bucket" json:"destination_bucket" binding:"required"`
		DestinationKey    string `form:"destination_key" json:"destination_key" binding:"required"`
	}
	var request CopyObjectRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	var copySource string
	if request.SourceKey[0] == '/' {
		copySource = request.SourceBucket + request.SourceKey
	} else {
		copySource = request.SourceBucket + "/" + request.SourceKey
	}
	client := createS3Client(c)
	_, err := client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket:     aws.String(request.DestinationBucket),
		CopySource: aws.String(copySource),
		Key:        aws.String(request.DestinationKey),
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		responseForPostForm(c, constant.Success, "复制成功", nil)
	}
}

func deleteObject(c *gin.Context) {
	type DeleteObjectRequest struct {
		Bucket string `form:"bucket" json:"bucket" binding:"required"`
		Key    string `form:"key" json:"key" binding:"required"`
	}
	var request DeleteObjectRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	client := createS3Client(c)
	_, err := client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(request.Bucket),
		Key:    aws.String(request.Key),
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		responseForPostForm(c, constant.Success, "删除成功", nil)
	}
}

func deleteObjects(c *gin.Context) {
	type DeleteObjectsRequest struct {
		DataFormat string   `form:"data_format" json:"data_format"`
		Bucket     string   `form:"bucket" json:"bucket" binding:"required"`
		Keys       []string `form:"keys" json:"keys" binding:"required"`
	}
	var request DeleteObjectsRequest
	if err := c.BindJSON(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	client := createS3Client(c)
	var identifiers = make([]types.ObjectIdentifier, 0)
	for _, v := range request.Keys {
		identifiers = append(identifiers, types.ObjectIdentifier{
			Key: aws.String(v),
		})
	}
	input := s3.DeleteObjectsInput{
		Bucket: aws.String(request.Bucket),
		Delete: &types.Delete{
			Objects: identifiers,
		},
	}
	_, err := client.DeleteObjects(context.Background(), &input)
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println(err.Error())
	} else {
		response(c, constant.Success, "删除成功", nil, request.DataFormat)
	}
}

func moveObject(c *gin.Context) {
	type CopyObjectRequest struct {
		SourceBucket      string `form:"source_bucket" json:"source_bucket" binding:"required"`
		SourceKey         string `form:"source_key" json:"source_key" binding:"required"`
		DestinationBucket string `form:"destination_bucket" json:"destination_bucket" binding:"required"`
		DestinationKey    string `form:"destination_key" json:"destination_key" binding:"required"`
	}
	var request CopyObjectRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	var copySource string
	if request.SourceKey[0] == '/' {
		copySource = request.SourceBucket + request.SourceKey
	} else {
		copySource = request.SourceBucket + "/" + request.SourceKey
	}
	client := createS3Client(c)
	_, err := client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket:     aws.String(request.DestinationBucket),
		CopySource: aws.String(copySource),
		Key:        aws.String(request.DestinationKey),
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		_, deleteSourceErr := client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
			Bucket: aws.String(request.SourceBucket),
			Key:    aws.String(request.SourceKey),
		})
		if deleteSourceErr != nil {
			rollbackRetryCount := 3
			for i := 0; i < rollbackRetryCount; i++ {
				_, deleteCopyErr := client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
					Bucket: aws.String(request.DestinationBucket),
					Key:    aws.String(request.DestinationKey),
				})
				if deleteCopyErr != nil {
					if i == rollbackRetryCount-1 {
						responseForPostForm(c, constant.UnknownError, err.Error(), nil)
						println(err.Error())
					}
				} else {
					responseForPostForm(c, constant.UnknownError, deleteSourceErr.Error(), nil)
					return
				}
			}
		} else {
			responseForPostForm(c, constant.Success, "移动成功", nil)
		}
	}
}

func uploadObject(c *gin.Context) {
	type UploadObjectRequest struct {
		Bucket string `form:"bucket" json:"bucket" binding:"required"`
		Key    string `form:"key" json:"key" binding:"required"`
	}
	var request UploadObjectRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	}
	client := createS3Client(c)
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = 64 * 1024 * 1024
	})
	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(request.Bucket),
		Key:    aws.String(request.Key),
		Body:   file,
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		responseForPostForm(c, constant.Success, "上传成功", nil)
	}
}

func downloadObject(c *gin.Context) {
	type DownloadObjectRequest struct {
		Bucket string `form:"bucket" json:"bucket" binding:"required"`
		Key    string `form:"key" json:"key" binding:"required"`
	}
	var request DownloadObjectRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	client := createS3Client(c)
	downloader := manager.NewDownloader(client)
	w := &manager.WriteAtBuffer{}
	_, err := downloader.Download(context.TODO(), w, &s3.GetObjectInput{
		Bucket: aws.String(request.Bucket),
		Key:    aws.String(request.Key),
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		c.Data(http.StatusOK, "application/octet-stream", w.Bytes())
	}
}

func headObject(c *gin.Context) {
	type HeadObjectRequest struct {
		Bucket string `form:"bucket" json:"bucket" binding:"required"`
		Key    string `form:"key" json:"key" binding:"required"`
	}
	var request HeadObjectRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	client := createS3Client(c)
	result, err := client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(request.Bucket),
		Key:    aws.String(request.Key),
	})
	type HeadObjectResponse struct {
		ETag          string `json:"etag"`
		ContentLength int64  `json:"content_length"`
		LastModified  int64  `json:"last_modified"`
	}
	headObjectResponse := HeadObjectResponse{
		ETag:          *result.ETag,
		ContentLength: result.ContentLength,
		LastModified:  getFormatTime(result.LastModified),
	}
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		responseForPostForm(c, constant.Success, "success", headObjectResponse)
	}
}

func getObjectTagging(c *gin.Context) {
	type GetObjectTaggingRequest struct {
		Bucket string `form:"bucket" json:"bucket" binding:"required"`
		Key    string `form:"key" json:"key" binding:"required"`
	}
	var request GetObjectTaggingRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	client := createS3Client(c)
	result, err := client.GetObjectTagging(context.TODO(), &s3.GetObjectTaggingInput{
		Bucket: aws.String(request.Bucket),
		Key:    aws.String(request.Key),
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		type tag struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		tags := make([]tag, 0)
		for _, v := range result.TagSet {
			var t tag
			t.Key = *v.Key
			t.Value = *v.Value
			tags = append(tags, t)
		}
		responseForPostForm(c, constant.Success, "success", tags)
	}
}

func putObjectTagging(c *gin.Context) {
	type Tag struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	type PutObjectTaggingRequest struct {
		DataFormat string `json:"data_format"`
		Bucket     string `json:"bucket" binding:"required"`
		Key        string `json:"key" binding:"required"`
		Tags       []Tag  `json:"tags"`
	}
	var p PutObjectTaggingRequest
	if err := c.BindJSON(&p); err != nil {
		handleValidationError(c, err)
		return
	}
	var tagging types.Tagging
	for _, v := range p.Tags {
		println(v.Key)
		tagging.TagSet = append(tagging.TagSet, types.Tag{
			Key:   aws.String(v.Key),
			Value: aws.String(v.Value),
		})
	}
	client := createS3Client(c)
	_, err := client.PutObjectTagging(context.TODO(), &s3.PutObjectTaggingInput{
		Bucket:  aws.String(p.Bucket),
		Key:     aws.String(p.Key),
		Tagging: &tagging,
	})
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, p.DataFormat)
		println(err.Error())
	} else {
		response(c, constant.Success, "添加成功", nil, p.DataFormat)
	}
}

func deleteObjectTagging(c *gin.Context) {
	type DeleteObjectTagging struct {
		Bucket string `form:"bucket" json:"bucket" binding:"required"`
		Key    string `form:"key" json:"key" binding:"required"`
	}
	var request DeleteObjectTagging
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	client := createS3Client(c)
	_, err := client.DeleteObjectTagging(context.TODO(), &s3.DeleteObjectTaggingInput{
		Bucket: aws.String(request.Bucket),
		Key:    aws.String(request.Key),
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		responseForPostForm(c, constant.Success, "删除成功", nil)
	}
}

func createMultipartUpload(c *gin.Context) {
	type CreateMultipartUploadRequest struct {
		Bucket string `form:"bucket" json:"bucket" binding:"required"`
		Key    string `form:"key" json:"key" binding:"required"`
	}
	var request CreateMultipartUploadRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	client := createS3Client(c)
	result, err := client.CreateMultipartUpload(context.TODO(), &s3.CreateMultipartUploadInput{
		Bucket: aws.String(request.Bucket),
		Key:    aws.String(request.Key),
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		type output struct {
			UploadId string `json:"upload_id"`
		}
		var o output
		o.UploadId = *result.UploadId
		responseForPostForm(c, constant.Success, "success", o)
	}
}

func abortMultipartUpload(c *gin.Context) {
	type AbortMultipartUploadRequest struct {
		Bucket   string `form:"bucket" json:"bucket" binding:"required"`
		Key      string `form:"key" json:"key" binding:"required"`
		UploadId string `form:"upload_id" json:"upload_id" binding:"required"`
	}
	var request AbortMultipartUploadRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	client := createS3Client(c)
	_, err := client.AbortMultipartUpload(context.TODO(), &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(request.Bucket),
		Key:      aws.String(request.Key),
		UploadId: aws.String(request.UploadId),
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		responseForPostForm(c, constant.Success, "success", nil)
	}
}

func completeMultipartUpload(c *gin.Context) {
	client := createS3Client(c)
	type Part struct {
		PartNumber int32  `json:"part_number" binding:"required"`
		ETag       string `json:"etag" binding:"required"`
	}
	type CompletedMultipartUploadRequest struct {
		DataFormat string `json:"data_format"`
		Bucket     string `json:"bucket" binding:"required"`
		Key        string `json:"key" binding:"required"`
		UploadId   string `json:"upload_id" binding:"required"`
		Parts      []Part `json:"parts" binding:"required"`
	}
	var request CompletedMultipartUploadRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	var multipartUpload types.CompletedMultipartUpload
	var parts []types.CompletedPart
	for _, v := range request.Parts {
		var p types.CompletedPart
		p.PartNumber = v.PartNumber
		p.ETag = aws.String(v.ETag)
		parts = append(parts, p)
	}
	multipartUpload.Parts = parts
	_, err := client.CompleteMultipartUpload(context.TODO(), &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(request.Bucket),
		Key:             aws.String(request.Key),
		UploadId:        aws.String(request.UploadId),
		MultipartUpload: &multipartUpload,
	})
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println(err.Error())
	} else {
		response(c, constant.Success, "success", nil, request.DataFormat)
	}
}

func listMultipartUploads(c *gin.Context) {
	type ListMultipartUploadsRequest struct {
		Bucket string `form:"bucket" json:"bucket" binding:"required"`
	}
	var request ListMultipartUploadsRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	client := createS3Client(c)
	result, err := client.ListMultipartUploads(context.TODO(), &s3.ListMultipartUploadsInput{
		Bucket: aws.String(request.Bucket),
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		type multipartUpload struct {
			Initiated int64  `json:"initiated"`
			Key       string `json:"key"`
			UploadId  string `json:"upload_id"`
		}
		multipartUploads := make([]multipartUpload, 0)
		for _, v := range result.Uploads {
			var t multipartUpload
			t.Initiated = getFormatTime(v.Initiated)
			t.Key = *v.Key
			t.UploadId = *v.UploadId
			multipartUploads = append(multipartUploads, t)
		}
		responseForPostForm(c, constant.Success, "success", multipartUploads)
	}
}

func listParts(c *gin.Context) {
	type ListPartsRequest struct {
		Bucket   string `form:"bucket" json:"bucket" binding:"required"`
		Key      string `form:"key" json:"key" binding:"required"`
		UploadId string `form:"upload_id" json:"upload_id" binding:"required"`
	}
	var request ListPartsRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	client := createS3Client(c)
	result, err := client.ListParts(context.TODO(), &s3.ListPartsInput{
		Bucket:   aws.String(request.Bucket),
		Key:      aws.String(request.Key),
		UploadId: aws.String(request.UploadId),
	})
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		type part struct {
			PartNumber int32  `json:"part_number"`
			Size       int64  `json:"size"`
			ETag       string `json:"etag"`
		}
		parts := make([]part, 0)
		for _, v := range result.Parts {
			var t part
			t.PartNumber = v.PartNumber
			t.Size = v.Size
			t.ETag = *v.ETag
			parts = append(parts, t)
		}
		responseForPostForm(c, constant.Success, "success", parts)
	}
}

func uploadPart(c *gin.Context) {
	type UploadPartRequest struct {
		Bucket     string `form:"bucket" json:"bucket" binding:"required"`
		Key        string `form:"key" json:"key" binding:"required"`
		PartNumber int32  `form:"part_number" json:"part_number" binding:"required"`
		UploadId   string `form:"upload_id" json:"upload_id" binding:"required"`
	}
	var request UploadPartRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	}
	client := createS3Client(c)
	result, err := client.UploadPart(context.TODO(), &s3.UploadPartInput{
		Bucket:     aws.String(request.Bucket),
		Key:        aws.String(request.Key),
		PartNumber: request.PartNumber,
		UploadId:   aws.String(request.UploadId),
		Body:       file,
	})
	type UploadPartResult struct {
		ETag string `json:"etag"`
	}
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		responseForPostForm(c, constant.Success, "上传成功", UploadPartResult{ETag: *result.ETag})
	}
}

func shareUrl(c *gin.Context) {
	type ShareRequest struct {
		Bucket           string `form:"bucket" json:"bucket" binding:"required"`
		Key              string `form:"key" json:"key" binding:"required"`
		SatelliteNodeURL string `form:"satellite_node_url" json:"satellite_node_url" binding:"required"`
		ApiKey           string `form:"api_key" json:"api_key" binding:"required"`
		Password         string `form:"password" json:"password" binding:"required"`
		ProjectId        string `form:"project_id" json:"project_id" binding:"required"`
		AuthService      string `form:"auth_service" json:"auth_service" binding:"required"`
		BaseUrl          string `form:"base_url" json:"base_url" binding:"required"`
		NotBefore        int64  `form:"not_before" json:"not_before"`
		NotAfter         int64  `form:"not_after" json:"not_after"`
	}
	var request ShareRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}

	accessGrant, err := console.GenAccessGrant(request.SatelliteNodeURL,
		request.ApiKey,
		request.Password,
		request.ProjectId,
	)
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
		return
	}

	var path string
	if request.Key[0] == '/' {
		path = request.Bucket + request.Key
	} else {
		path = request.Bucket + "/" + request.Key
	}
	paths := make([]string, 0)
	paths = append(paths, path)
	var notBefore, notAfter time.Time
	if request.NotBefore != 0 {
		notBefore = time.UnixMilli(request.NotBefore)
	}
	if request.NotAfter != 0 {
		notAfter = time.UnixMilli(request.NotAfter)
	}
	permission := console.Permission{
		AllowDownload: true,
		AllowDelete:   false,
		AllowList:     true,
		AllowUpload:   false,
		NotBefore:     notBefore,
		NotAfter:      notAfter,
	}
	restrictGrant, _ := console.RestrictGrant(accessGrant, paths, permission)
	cs, err := getCredentials(request.AuthService, restrictGrant, true)
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		responseForPostForm(c, constant.Success, "success", request.BaseUrl+"/"+cs.AccessKeyId+"/"+url.QueryEscape(path))
	}
}

func createRestrictKey(c *gin.Context) {
	type CreateRestrictKeyRequest struct {
		DataFormat    string   `form:"data_format" json:"data_format"`
		Buckets       []string `form:"buckets" json:"buckets"`
		ApiKey        string   `form:"api_key" json:"api_key" binding:"required"`
		AllowDownload bool     `form:"allow_download" json:"allow_download"`
		AllowDelete   bool     `form:"allow_delete" json:"allow_delete"`
		AllowList     bool     `form:"allow_list" json:"allow_list"`
		AllowUpload   bool     `form:"allow_upload" json:"allow_upload"`
		NotBefore     int64    `form:"not_before" json:"not_before"`
		NotAfter      int64    `form:"not_after" json:"not_after"`
	}
	var request CreateRestrictKeyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	restrictedKey, err := restrictKey(request.ApiKey, request.Buckets, request.AllowDownload, request.AllowDelete, request.AllowList, request.AllowUpload,
		request.NotBefore, request.NotAfter)
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println(err.Error())
	} else {
		response(c, constant.Success, "success", restrictedKey, request.DataFormat)
	}
}

func restrictKey(APIKey string, buckets []string, allowDownload, allowDelete, allowList, allowUpload bool, notBefore, notAfter int64) (string, error) {
	var notBeforeTime, notAfterTime time.Time
	if notBefore != 0 {
		notBeforeTime = time.UnixMilli(notBefore)
	}
	if notAfter != 0 {
		notAfterTime = time.UnixMilli(notAfter)
	}
	permission := console.Permission{
		AllowDownload: allowDownload,
		AllowDelete:   allowDelete,
		AllowList:     allowList,
		AllowUpload:   allowUpload,
		NotBefore:     notBeforeTime,
		NotAfter:      notAfterTime,
	}
	restrictedKey, err := console.SetPermission(APIKey, buckets, permission)
	return restrictedKey.Serialize(), err
}

func createAccessGrant(c *gin.Context) {
	type CreateAccessGrantRequest struct {
		SatelliteNodeURL string `form:"satellite_node_url" json:"satellite_node_url" binding:"required"`
		RestrictKey      string `form:"restrict_key" json:"restrict_key" binding:"required"`
		Password         string `form:"password" json:"password" binding:"required"`
		ProjectId        string `form:"project_id" json:"project_id" binding:"required"`
	}
	var request CreateAccessGrantRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	access, err := accessGrant(request.SatelliteNodeURL, request.RestrictKey, request.Password, request.ProjectId)
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		responseForPostForm(c, constant.Success, "success", access)
	}
}

func accessGrant(satelliteNodeURL, restrictKey, password, projectId string) (string, error) {
	return console.GenAccessGrant(satelliteNodeURL,
		restrictKey,
		password,
		projectId,
	)
}

func createS3Client(c *gin.Context) *s3.Client {
	creds := credentials.NewStaticCredentialsProvider(c.GetHeader("AccessKeyId"), c.GetHeader("SecretAccessKey"), "")
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: c.GetHeader("Endpoint"),
		}, nil
	})
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithCredentialsProvider(creds), config.WithEndpointResolverWithOptions(customResolver))
	if err != nil {
		println("failed to load SDK configuration, %v", err)
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
}

func responseForPostForm(c *gin.Context, code int, msg, data any) {
	response(c, code, msg, data, c.PostForm("data_format"))
}

func response(c *gin.Context, code int, msg, data any, dataFormat string) {
	var h any
	if data == nil {
		h = gin.H{"code": code, "msg": msg}
	} else {
		h = gin.H{"code": code, "msg": msg, "data": data}
	}
	if strings.ToLower(dataFormat) == "xml" {
		c.XML(http.StatusOK, h)
	} else {
		c.JSON(http.StatusOK, h)
	}
}

func getFormatTime(time *time.Time) int64 {
	return time.UnixNano() / 1e6
}

type Credentials struct {
	AccessKeyId     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	Endpoint        string `json:"endpoint"`
}

func getCredentials(authService, accessGrant string, public bool) (Credentials, error) {
	postData, err := json.Marshal(map[string]interface{}{
		"access_grant": accessGrant,
		"public":       public,
	})
	if err != nil {
		return Credentials{}, err
	}
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/v1/access", authService), bytes.NewReader(postData))
	if err != nil {
		return Credentials{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return Credentials{}, err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()
	body, err := ioutil.ReadAll(resp.Body)
	println("getCredentials statusCode:" + strconv.Itoa(resp.StatusCode) + ", body:" + string(body))
	if err != nil {
		return Credentials{}, err
	}
	if resp.StatusCode != 200 {
		return Credentials{}, errs.New(string(body))
	} else {
		type Access struct {
			AccessKeyId     string `json:"access_key_id"`
			SecretAccessKey string `json:"secret_key"`
			Endpoint        string `json:"endpoint"`
		}
		var access Access
		if err := json.Unmarshal(body, &access); err != nil {
			return Credentials{}, err
		}
		var cs Credentials
		cs.AccessKeyId = access.AccessKeyId
		cs.SecretAccessKey = access.SecretAccessKey
		cs.Endpoint = access.Endpoint
		return cs, nil
	}
}

func createCredentialsByAccount(c *gin.Context) {
	type CreateCredentialsByAccountRequest struct {
		DataFormat          string `form:"data_format" json:"data_format"`
		Email               string `form:"email" json:"email" binding:"required"`
		LoginPassword       string `form:"login_password" json:"login_password" binding:"required"`
		CredentialsPassword string `form:"credentials_password" json:"credentials_password" binding:"required"`
		SatelliteNodeURL    string `form:"satellite_node_url" json:"satellite_node_url" binding:"required"`
		CloudService        string `form:"cloud_service" json:"cloud_service" binding:"required"`
		AuthService         string `form:"auth_service" json:"auth_service" binding:"required"`
	}
	var request CreateCredentialsByAccountRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	_, err := register(request.CloudService, request.Email, request.LoginPassword, request.Email[0:strings.Index(request.Email, "@")])
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println("register:" + err.Error())
		return
	}
	time.Sleep(time.Millisecond * 100)
	token, err := login(request.CloudService, request.Email, request.LoginPassword)
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println("login:" + err.Error())
		return
	}
	println("token:" + token)
	projects, err := getProjects(request.CloudService, token)
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println("getProjects:" + err.Error())
		return
	}
	var projectId string
	if projects == nil || len(projects) == 0 {
		proId, err := createProject(request.CloudService, token, "My First Project")
		if err != nil {
			response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
			println("createProject:" + err.Error())
			return
		}
		projectId = proId
	} else {
		projectId = projects[0].Id
	}
	apiKey, err := createAPIKey(request.CloudService, token, projectId, "backend-"+time.Now().String())
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println("createAPIKey:" + err.Error())
		return
	}
	println("apiKey:" + apiKey)
	restrictKey, err := restrictKey(apiKey, nil, true, true, true, true, 0, 0)
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println("restrictKey:" + err.Error())
		return
	}
	println("restrictKey:" + restrictKey)
	accessGrant, err := accessGrant(request.SatelliteNodeURL, restrictKey, request.CredentialsPassword, projectId)
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println("accessGrant:" + err.Error())
		return
	}
	println("accessGrant:" + accessGrant)
	cs, err := getCredentials(request.AuthService, accessGrant, false)
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println("getCredentials:" + err.Error())
		return
	}
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		responseForPostForm(c, constant.Success, "success", cs)
	}
}

func getAccessGrantByAccount(c *gin.Context) {
	type CreateCredentialsByAccountRequest struct {
		DataFormat          string `form:"data_format" json:"data_format"`
		Email               string `form:"email" json:"email" binding:"required"`
		LoginPassword       string `form:"login_password" json:"login_password" binding:"required"`
		CredentialsPassword string `form:"credentials_password" json:"credentials_password" binding:"required"`
		SatelliteNodeURL    string `form:"satellite_node_url" json:"satellite_node_url" binding:"required"`
		CloudService        string `form:"cloud_service" json:"cloud_service" binding:"required"`
		AuthService         string `form:"auth_service" json:"auth_service" binding:"required"`
	}
	var request CreateCredentialsByAccountRequest
	if err := c.ShouldBind(&request); err != nil {
		handleValidationError(c, err)
		return
	}
	_, err := register(request.CloudService, request.Email, request.LoginPassword, request.Email[0:strings.Index(request.Email, "@")])
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println("register:" + err.Error())
		return
	}
	time.Sleep(time.Millisecond * 100)
	token, err := login(request.CloudService, request.Email, request.LoginPassword)
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println("login:" + err.Error())
		return
	}
	println("token:" + token)
	projects, err := getProjects(request.CloudService, token)
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println("getProjects:" + err.Error())
		return
	}
	var projectId string
	if projects == nil || len(projects) == 0 {
		proId, err := createProject(request.CloudService, token, "My First Project")
		if err != nil {
			response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
			println("createProject:" + err.Error())
			return
		}
		projectId = proId
	} else {
		projectId = projects[0].Id
	}
	apiKey, err := createAPIKey(request.CloudService, token, projectId, "backend-"+time.Now().String())
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println("createAPIKey:" + err.Error())
		return
	}
	println("apiKey:" + apiKey)
	restrictKey, err := restrictKey(apiKey, nil, true, true, true, true, 0, 0)
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println("restrictKey:" + err.Error())
		return
	}
	println("restrictKey:" + restrictKey)
	accessGrant, err := accessGrant(request.SatelliteNodeURL, restrictKey, request.CredentialsPassword, projectId)
	if err != nil {
		response(c, constant.UnknownError, err.Error(), nil, request.DataFormat)
		println("accessGrant:" + err.Error())
		return
	}
	println("accessGrant:" + accessGrant)
	if err != nil {
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		println(err.Error())
	} else {
		responseForPostForm(c, constant.Success, "success", accessGrant)
	}
}

func register(host, email, password, fullName string) (string, error) {
	postData, err := json.Marshal(map[string]interface{}{
		"email":    email,
		"password": password,
		"fullName": fullName,
	})
	if err != nil {
		return "", err
	}
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v0/auth/register", host), bytes.NewReader(postData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", errors.New(string(body))
	}
	return string(body), nil
}

func login(host, email, password string) (string, error) {
	postData, err := json.Marshal(map[string]interface{}{
		"email":    email,
		"password": password,
	})
	if err != nil {
		return "", err
	}
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v0/auth/token", host), bytes.NewReader(postData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	println("login body:" + string(body) + ", " + strconv.Itoa(resp.StatusCode))
	if resp.StatusCode != 200 {
		return "", errors.New(string(body))
	} else {
		return string(body), nil
	}
}

type ProjectListData struct {
	MyProjects []Project `json:"myProjects"`
}

type Project struct {
	CreatedAt   string `json:"createdAt"`
	Description string `json:"description"`
	Id          string `json:"id"`
	Name        string `json:"name"`
	OwnerId     string `json:"ownerId"`
}

func getProjects(host, token string) ([]Project, error) {
	param := "{\"operationName\":null,\"variables\":{},\"query\":\"{\\n  myProjects {\\n    name\\n    id\\n    description\\n    createdAt\\n    ownerId\\n    __typename\\n  }\\n}\\n\"}"
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v0/graphql", host), bytes.NewReader([]byte(param)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	cookie := http.Cookie{
		Name: "_tokenKey", Value: token,
	}
	req.AddCookie(&cookie)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	println("getProjects body:" + string(body))
	type HttpResult struct {
		Error string          `json:"error"`
		Data  ProjectListData `json:"data"`
	}
	var httpResult HttpResult
	if err := json.Unmarshal(body, &httpResult); err != nil {
		return nil, err
	}
	if len(httpResult.Error) > 0 {
		return nil, errors.New(httpResult.Error)
	}
	return httpResult.Data.MyProjects, nil
}

type Data struct {
	APIKey CreateAPIKey `json:"createAPIKey"`
}

type CreateAPIKey struct {
	Key string `json:"key"`
}

func createAPIKey(host, token, projectId, name string) (string, error) {
	param := "{\"operationName\":null,\"variables\":{\"projectId\":\"" + projectId +
		"\",\"name\":\"" + name + "\"},\"query\":\"mutation ($projectId: String!, $name: String!) {\\n  createAPIKey(projectID: $projectId, name: $name) " +
		"{\\n    key\\n    keyInfo {\\n      id\\n      name\\n      createdAt\\n      __typename\\n    }\\n    __typename\\n  }\\n}\\n\"}"
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v0/graphql", host), bytes.NewReader([]byte(param)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	cookie := http.Cookie{
		Name: "_tokenKey", Value: token,
	}
	req.AddCookie(&cookie)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	println("createAPIKey statusCode:" + strconv.Itoa(resp.StatusCode) + ", body:" + string(body))
	if resp.StatusCode != 200 {
		return "", errors.New(string(body))
	}
	type HttpResult struct {
		Data Data `json:"data"`
	}
	var httpResult HttpResult
	if err := json.Unmarshal(body, &httpResult); err != nil {
		return "", err
	}
	return httpResult.Data.APIKey.Key, nil
}

func createProject(host, token, projectName string) (string, error) {
	param := "{\"operationName\":null,\"variables\":{\"name\":\"" + projectName + "\",\"description\":\"___\"},\"query\":\"mutation ($name: String!, $description: String!) " +
		"{\\n  createProject(input: {name: $name, description: $description}) {\\n    id\\n    __typename\\n  }\\n}\\n\"}"
	/*param := "{\"operationName\":null,\"variables\":{\"name\":\"My First Project\",\"description\":\"___\"},\"query\":\"mutation ($name: String!, $description: String!) " +
	"{\\n  createProject(input: {name: $name, description: $description}) {\\n    id\\n    __typename\\n  }\\n}\\n\"}"*/
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v0/graphql", host), bytes.NewReader([]byte(param)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	cookie := http.Cookie{
		Name: "_tokenKey", Value: token,
	}
	req.AddCookie(&cookie)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	println("createProject statusCode:" + strconv.Itoa(resp.StatusCode) + ", body:" + string(body))
	if resp.StatusCode != 200 {
		return "", errors.New(string(body))
	}
	type CreateProjectData struct {
		CreateProject Project `json:"createProject"`
	}
	type HttpResult struct {
		Data CreateProjectData `json:"data"`
	}
	var httpResult HttpResult
	if err := json.Unmarshal(body, &httpResult); err != nil {
		return "", err
	}
	return httpResult.Data.CreateProject.Id, nil
}

// 定义一个全局翻译器T
var trans ut.Translator

// InitTrans 初始化翻译器
func InitTrans(locale string) (err error) {
	// 修改gin框架中的Validator引擎属性，实现自定制
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {

		zhT := zh.New() // 中文翻译器
		enT := en.New() // 英文翻译器

		// 第一个参数是备用（fallback）的语言环境
		// 后面的参数是应该支持的语言环境（支持多个）
		// uni := ut.New(zhT, zhT) 也是可以的
		uni := ut.New(enT, zhT, enT)

		// locale 通常取决于 http 请求头的 'Accept-Language'
		var ok bool
		// 也可以使用 uni.FindTranslator(...) 传入多个locale进行查找
		trans, ok = uni.GetTranslator(locale)
		if !ok {
			return fmt.Errorf("uni.GetTranslator(%s) failed", locale)
		}

		// 注册翻译器
		switch locale {
		case "en":
			err = enTranslations.RegisterDefaultTranslations(v, trans)
		case "zh":
			err = zhTranslations.RegisterDefaultTranslations(v, trans)
		default:
			err = enTranslations.RegisterDefaultTranslations(v, trans)
		}
		return
	}
	return
}

func handleValidationError(c *gin.Context, err error) {
	ss, ok := err.(validator.ValidationErrors)
	if !ok {
		// 非validator.ValidationErrors类型错误直接返回
		responseForPostForm(c, constant.UnknownError, err.Error(), nil)
		return
	}
	// validator.ValidationErrors类型错误则进行翻译
	responseForPostForm(c, constant.UnknownError, formatValidationErrorMsg(ss.Translate(trans)), nil)
}

func formatValidationErrorMsg(fields map[string]string) string {
	var e string
	i := 0
	for _, v := range fields {
		if i == 0 {
			e = v
		} else {
			e = e + ";" + v
		}
		i++
	}
	return e
}
