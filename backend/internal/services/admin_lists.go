package services

import (
	"context"
	"errors"
	"sort"
	"sync"

	"Mimic890/kuraban/internal/models"
)

// BucketWithInfo is a listed bucket and its details. Info is nil when the details
// could not be read.
type BucketWithInfo struct {
	Bucket models.ListBucketsResponseItem
	Info   *models.GarageBucketInfo
}

// ListBucketsWithInfo returns every bucket with its details, in a batch when
// Garage offers one and otherwise with one request per bucket. Running out of time
// is an error: a list that just stops early would look complete.
func ListBucketsWithInfo(ctx context.Context, admin AdminService) ([]BucketWithInfo, error) {
	infos, err := admin.ListBucketsInfo(ctx)
	if err == nil {
		out := make([]BucketWithInfo, len(infos))
		for i := range infos {
			out[i] = BucketWithInfo{Bucket: listItemFromInfo(&infos[i]), Info: &infos[i]}
		}
		return out, nil
	}
	if !errors.Is(err, ErrUnsupported) {
		return nil, err
	}

	buckets, err := admin.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]BucketWithInfo, len(buckets))
	forEachLimited(len(buckets), func(i int) {
		info, _ := admin.GetBucketInfo(ctx, buckets[i].ID)
		out[i] = BucketWithInfo{Bucket: buckets[i], Info: info}
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// listItemFromInfo rebuilds what ListBuckets would have said about a bucket. Its
// local aliases come sorted the same way, because they name the bucket's metric
// series and a different pick would split its history.
func listItemFromInfo(info *models.GarageBucketInfo) models.ListBucketsResponseItem {
	item := models.ListBucketsResponseItem{ID: info.ID, Created: info.Created, GlobalAliases: info.GlobalAliases}
	for _, key := range info.Keys {
		for _, alias := range key.BucketLocalAliases {
			item.LocalAliases = append(item.LocalAliases, models.BucketLocalAlias{AccessKeyID: key.AccessKeyID, Alias: alias})
		}
	}
	sort.Slice(item.LocalAliases, func(i, j int) bool {
		a, b := item.LocalAliases[i], item.LocalAliases[j]
		if a.AccessKeyID != b.AccessKeyID {
			return a.AccessKeyID < b.AccessKeyID
		}
		return a.Alias < b.Alias
	})
	return item
}

// ListKeysWithInfo returns every access key with its details and no secrets,
// leaving out keys whose details could not be read. See ListBucketsWithInfo.
func ListKeysWithInfo(ctx context.Context, admin AdminService) ([]models.GarageKeyInfo, error) {
	keys, err := admin.ListKeysInfo(ctx)
	if err == nil || !errors.Is(err, ErrUnsupported) {
		return keys, err
	}

	list, err := admin.ListKeys(ctx)
	if err != nil {
		return nil, err
	}
	details := make([]*models.GarageKeyInfo, len(list))
	forEachLimited(len(list), func(i int) {
		details[i], _ = admin.GetKeyInfo(ctx, list[i].ID, false)
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	keys = make([]models.GarageKeyInfo, 0, len(list))
	for _, info := range details {
		if info != nil {
			keys = append(keys, *info)
		}
	}
	return keys, nil
}

// forEachLimited calls fn(0..n-1) with a few running at once: one at a time, a
// thousand items outlast the request timeout.
func forEachLimited(n int, fn func(i int)) {
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			fn(i)
		}()
	}
	wg.Wait()
}
