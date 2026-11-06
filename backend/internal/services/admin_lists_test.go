package services_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/internal/services"
	"Mimic890/kuraban/internal/services/mocks"
)

func TestListBucketsWithInfo_BatchMakesNoPerBucketCalls(t *testing.T) {
	admin := &mocks.AdminMock{
		ListBucketsInfoFn: func(context.Context) ([]models.GarageBucketInfo, error) {
			return []models.GarageBucketInfo{
				{ID: "1", GlobalAliases: []string{"alpha"}, Objects: 3},
				{ID: "2", Objects: 5, Keys: []models.BucketKeyInfo{
					{AccessKeyID: "GKb", BucketLocalAliases: []string{"z"}},
					{AccessKeyID: "GKa", BucketLocalAliases: []string{"y", "x"}},
				}},
			}, nil
		},
		GetBucketInfoFn: func(_ context.Context, id string) (*models.GarageBucketInfo, error) {
			t.Errorf("per-bucket call for %s", id)
			return nil, nil
		},
	}
	got, err := services.ListBucketsWithInfo(context.Background(), admin)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Info.Objects != 3 || got[1].Info.Objects != 5 || got[0].Bucket.GlobalAliases[0] != "alpha" {
		t.Fatalf("got %+v", got)
	}
	aliases := got[1].Bucket.LocalAliases
	if len(aliases) != 3 || aliases[0].Alias != "x" || aliases[1].Alias != "y" || aliases[2].Alias != "z" || aliases[0].AccessKeyID != "GKa" {
		t.Errorf("local aliases = %+v, want sorted like ListBuckets", aliases)
	}
}

func TestListWithInfo_RealFailureDoesNotFallBack(t *testing.T) {
	boom := errors.New("boom")
	admin := &mocks.AdminMock{
		ListBucketsInfoFn: func(context.Context) ([]models.GarageBucketInfo, error) { return nil, boom },
		ListKeysInfoFn:    func(context.Context) ([]models.GarageKeyInfo, error) { return nil, boom },
		ListBucketsFn: func(context.Context) ([]models.ListBucketsResponseItem, error) {
			t.Error("fell back on a real failure")
			return nil, nil
		},
		ListKeysFn: func(context.Context) ([]models.ListKeysResponseItem, error) {
			t.Error("fell back on a real failure")
			return nil, nil
		},
	}
	if _, err := services.ListBucketsWithInfo(context.Background(), admin); !errors.Is(err, boom) {
		t.Errorf("buckets err = %v", err)
	}
	if _, err := services.ListKeysWithInfo(context.Background(), admin); !errors.Is(err, boom) {
		t.Errorf("keys err = %v", err)
	}
}

func TestListWithInfo_FallbackKeepsOrderAndToleratesMissingDetails(t *testing.T) {
	admin := &mocks.AdminMock{
		ListBucketsFn: func(context.Context) ([]models.ListBucketsResponseItem, error) {
			list := make([]models.ListBucketsResponseItem, 50)
			for i := range list {
				list[i] = models.ListBucketsResponseItem{ID: strconv.Itoa(i)}
			}
			return list, nil
		},
		GetBucketInfoFn: func(_ context.Context, id string) (*models.GarageBucketInfo, error) {
			switch id {
			case "3":
				return nil, errors.New("forbidden")
			case "4":
				return nil, nil // deleted while listing
			}
			return &models.GarageBucketInfo{ID: id}, nil
		},
		ListKeysFn: func(context.Context) ([]models.ListKeysResponseItem, error) {
			return []models.ListKeysResponseItem{{ID: "a"}, {ID: "bad"}, {ID: "c"}}, nil
		},
		GetKeyInfoFn: func(_ context.Context, id string, showSecret bool) (*models.GarageKeyInfo, error) {
			if showSecret {
				t.Error("must not request secrets")
			}
			if id == "bad" {
				return nil, errors.New("forbidden")
			}
			return &models.GarageKeyInfo{AccessKeyID: id}, nil
		},
	}
	buckets, err := services.ListBucketsWithInfo(context.Background(), admin)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range buckets {
		if b.Bucket.ID != strconv.Itoa(i) {
			t.Fatalf("bucket %d = %s", i, b.Bucket.ID)
		}
		if withInfo := i != 3 && i != 4; withInfo != (b.Info != nil) {
			t.Errorf("bucket %d info = %v", i, b.Info)
		}
	}
	keys, err := services.ListKeysWithInfo(context.Background(), admin)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0].AccessKeyID != "a" || keys[1].AccessKeyID != "c" {
		t.Errorf("keys = %+v", keys)
	}
}

func TestListWithInfo_DeadlineIsAnErrorNotAShortList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	admin := &mocks.AdminMock{
		ListBucketsFn: func(context.Context) ([]models.ListBucketsResponseItem, error) {
			return []models.ListBucketsResponseItem{{ID: "1"}}, nil
		},
		GetBucketInfoFn: func(context.Context, string) (*models.GarageBucketInfo, error) {
			cancel()
			return nil, ctx.Err()
		},
		ListKeysFn: func(context.Context) ([]models.ListKeysResponseItem, error) {
			return []models.ListKeysResponseItem{{ID: "a"}}, nil
		},
		GetKeyInfoFn: func(context.Context, string, bool) (*models.GarageKeyInfo, error) {
			return nil, ctx.Err()
		},
	}
	if got, err := services.ListBucketsWithInfo(ctx, admin); err == nil {
		t.Errorf("buckets: want an error, got %d", len(got))
	}
	if got, err := services.ListKeysWithInfo(ctx, admin); err == nil {
		t.Errorf("keys: want an error, got %d", len(got))
	}
}
