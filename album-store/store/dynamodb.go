package store

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"album-store/models"
)

// DynamoDB wraps the AWS client and table names.
type DynamoDB struct {
	client      *dynamodb.Client
	albumsTable string
	photosTable string
}

// NewDynamoDB creates a DynamoDB client using the default credential chain
// (EC2 instance role, env vars, ~/.aws/credentials).
func NewDynamoDB(ctx context.Context) (*DynamoDB, error) {
	// Large connection pool — critical for load tests.
	transport := &http.Transport{
		MaxIdleConns:        2000,
		MaxIdleConnsPerHost: 1000,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	albumsTable := os.Getenv("ALBUMS_TABLE")
	if albumsTable == "" {
		albumsTable = "albums"
	}
	photosTable := os.Getenv("PHOTOS_TABLE")
	if photosTable == "" {
		photosTable = "photos"
	}

	return &DynamoDB{
		client:      dynamodb.NewFromConfig(cfg),
		albumsTable: albumsTable,
		photosTable: photosTable,
	}, nil
}

// ─── Albums ──────────────────────────────────────────────────────────────────

// PutAlbum upserts an album (idempotent).
func (d *DynamoDB) PutAlbum(ctx context.Context, album models.Album) error {
	item, err := attributevalue.MarshalMap(album)
	if err != nil {
		return err
	}
	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.albumsTable),
		Item:      item,
	})
	return err
}

// GetAlbum returns nil, nil when the album does not exist.
func (d *DynamoDB) GetAlbum(ctx context.Context, albumID string) (*models.Album, error) {
	result, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.albumsTable),
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
		},
		// Strong read so we always see data written moments ago.
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	if result.Item == nil {
		return nil, nil
	}
	var album models.Album
	if err := attributevalue.UnmarshalMap(result.Item, &album); err != nil {
		return nil, err
	}
	return &album, nil
}

// ListAlbums pages through a full Scan, returning every album ever created.
func (d *DynamoDB) ListAlbums(ctx context.Context) ([]models.Album, error) {
	var albums []models.Album
	var lastKey map[string]types.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName:      aws.String(d.albumsTable),
			ConsistentRead: aws.Bool(true),
		}
		if lastKey != nil {
			input.ExclusiveStartKey = lastKey
		}

		result, err := d.client.Scan(ctx, input)
		if err != nil {
			return nil, err
		}

		var page []models.Album
		if err := attributevalue.UnmarshalListOfMaps(result.Items, &page); err != nil {
			return nil, err
		}
		albums = append(albums, page...)

		if result.LastEvaluatedKey == nil {
			break
		}
		lastKey = result.LastEvaluatedKey
	}

	return albums, nil
}

// ─── Photo sequence counter ───────────────────────────────────────────────────

// IncrementAndGetSeq atomically increments the per-album photo counter
// and returns the new value as the seq number (starts at 1).
func (d *DynamoDB) IncrementAndGetSeq(ctx context.Context, albumID string) (int, error) {
	result, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(d.albumsTable),
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
		},
		UpdateExpression: aws.String("ADD photo_count :inc"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":inc": &types.AttributeValueMemberN{Value: "1"},
		},
		ReturnValues: types.ReturnValueUpdatedNew,
	})
	if err != nil {
		return 0, fmt.Errorf("increment seq: %w", err)
	}

	countAttr, ok := result.Attributes["photo_count"]
	if !ok {
		return 0, fmt.Errorf("photo_count missing from UpdateItem response")
	}
	countN, ok := countAttr.(*types.AttributeValueMemberN)
	if !ok {
		return 0, fmt.Errorf("photo_count is not a Number")
	}
	return strconv.Atoi(countN.Value)
}

// ─── Photos ───────────────────────────────────────────────────────────────────

// PutPhoto writes a new photo record.
func (d *DynamoDB) PutPhoto(ctx context.Context, photo models.Photo) error {
	item, err := attributevalue.MarshalMap(photo)
	if err != nil {
		return err
	}
	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.photosTable),
		Item:      item,
	})
	return err
}

// GetPhoto returns nil, nil when the photo does not exist.
func (d *DynamoDB) GetPhoto(ctx context.Context, albumID, photoID string) (*models.Photo, error) {
	result, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.photosTable),
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	if result.Item == nil {
		return nil, nil
	}
	var photo models.Photo
	if err := attributevalue.UnmarshalMap(result.Item, &photo); err != nil {
		return nil, err
	}
	return &photo, nil
}

// UpdatePhotoStatus sets status (and optionally url) on a photo.
func (d *DynamoDB) UpdatePhotoStatus(ctx context.Context, albumID, photoID, status, url string) error {
	updateExpr := "SET #s = :status"
	exprNames := map[string]string{"#s": "status"}
	exprValues := map[string]types.AttributeValue{
		":status": &types.AttributeValueMemberS{Value: status},
	}

	if url != "" {
		updateExpr += ", #u = :url"
		exprNames["#u"] = "url"
		exprValues[":url"] = &types.AttributeValueMemberS{Value: url}
	}

	// Only update if photo still exists and is in "processing" state.
	// This prevents the "lost delete" race: if the photo was deleted while being processed,
	// this update will fail instead of resurrecting the deleted record.
	exprValues[":processing"] = &types.AttributeValueMemberS{Value: "processing"}
	conditionExpr := "attribute_exists(photo_id) AND #s = :processing"

	_, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(d.photosTable),
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
		UpdateExpression:          aws.String(updateExpr),
		ConditionExpression:       aws.String(conditionExpr),
		ExpressionAttributeNames:  exprNames,
		ExpressionAttributeValues: exprValues,
	})
	return err
}

// DeletePhoto removes the photo metadata record.
func (d *DynamoDB) DeletePhoto(ctx context.Context, albumID, photoID string) error {
	_, err := d.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(d.photosTable),
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
	})
	return err
}