package storage

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

// Client wraps an S3-compatible bucket.
type Client struct {
	s3     *s3.Client
	bucket string
}

// NewClient inicializa um cliente compatível com S3 seguindo o padrão pkg/storage/s3.go

/*func NewClient(endpoint, region, accessKeyID, secretKey, bucketName string) (*Client, error) {
	// 1. Configuração do Transporte Customizado para evitar Connection Reset
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	customTransport.MaxIdleConns = 100
	customTransport.IdleConnTimeout = 90 * time.Second
	customTransport.TLSHandshakeTimeout = 10 * time.Second

	if region == "" || region == "auto" {
		region = "us-east-1"
	}

	// 2. Carrega a config base
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: load config: %w", err)
	}

	// 3. Criação do cliente com Options explícitas
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true // Essencial para provedores não-AWS
		o.HTTPClient = &http.Client{
			Timeout:   90 * time.Second,
			Transport: customTransport,
		}
	})

	return &Client{s3: s3Client, bucket: bucketName}, nil
}
*/

func NewClient(endpoint, region, accessKey, secretKey, bucket string) (*Client, error) {
	// 1. Normalização da Região
	if region == "" || region == "auto" {
		region = "us-east-1"
	}

	// 2. Transporte Customizado (Limpando as duplicatas)
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.MaxIdleConns = 10
	customTransport.IdleConnTimeout = 30 * time.Second
	customTransport.TLSHandshakeTimeout = 10 * time.Second
	customTransport.ForceAttemptHTTP2 = false // Mantemos desativado para estabilidade no T3

	httpClient := &http.Client{
		Transport: customTransport,
		Timeout:   120 * time.Second, // Timeout generoso para o OCR processar
	}

	// 3. Resolver de Endpoint
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:           endpoint,    // https://t3.storageapi.dev
			SigningRegion: "us-east-1", // Forçamos a região da assinatura
		}, nil
	})

	// 4. Configuração Base
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithHTTPClient(httpClient), // INJETAMOS O CLIENTE AQUI
	)
	if err != nil {
		return nil, err
	}

	// 5. Cliente S3
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.APIOptions = append(o.APIOptions,
			v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware,
		)
	})

	return &Client{
		s3:     s3Client,
		bucket: bucket,
	}, nil
}

// Upload armazena o arquivo e retorna a key.
func (c *Client) Upload(ctx context.Context, data []byte, mimeType string) (string, error) {
	key := uuid.New().String()

	// 4. PutObject otimizado
	input := &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(mimeType),
		ACL:         types.ObjectCannedACLPublicRead,
	}

	_, err := c.s3.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("storage: upload: %w", err)
	}

	return key, nil
}

// PublicURL gera a URL pública respeitando o PathStyle
func (c *Client) PublicURL(endpoint, key string) string {
	return fmt.Sprintf("%s/%s/%s", endpoint, c.bucket, key)
}

// PresignGetURL gera uma URL pré-assinada para download do objeto, válida pelo tempo especificado.
func (c *Client) PresignGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(c.s3)
	req, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("storage: presign: %w", err)
	}
	return req.URL, nil
}
