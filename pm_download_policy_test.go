package main

import (
	"testing"
)

func TestSelectEpisodesByDownloadPolicyAllCases(t *testing.T) {
	catalog := []FeedEpisode{
		{Title: "Ep 1", GUID: "g1", EnclosureURL: "http://example.com/1.mp3", PublishedAt: 1000},
		{Title: "Ep 2", GUID: "g2", EnclosureURL: "http://example.com/2.mp3", PublishedAt: 2000},
		{Title: "Ep 3", GUID: "g3", EnclosureURL: "http://example.com/3.mp3", PublishedAt: 3000},
		{Title: "Ep 4", GUID: "g4", EnclosureURL: "http://example.com/4.mp3", PublishedAt: 4000},
		{Title: "Ep 5", GUID: "g5", EnclosureURL: "http://example.com/5.mp3", PublishedAt: 5000},
	}

	downloaded := make(map[string]bool)
	isDownloaded := func(ep FeedEpisode) bool {
		return downloaded[ep.GUID]
	}

	eps, _ := selectEpisodesByDownloadPolicy(catalog, isDownloaded, "none", 0, false)
	if len(eps) != 0 {
		t.Errorf("none: expected 0, got %d", len(eps))
	}

	eps, reasons := selectEpisodesByDownloadPolicy(catalog, isDownloaded, "latest", 0, false)
	if len(eps) != 1 || eps[0].GUID != "g5" {
		t.Errorf("latest: expected [g5], got %+v", eps)
	}
	if len(reasons) == 0 || reasons[0] != "1 latest episode (policy: latest)" {
		t.Errorf("latest: unexpected reasons %+v", reasons)
	}

	downloaded["g5"] = true
	eps, _ = selectEpisodesByDownloadPolicy(catalog, isDownloaded, "latest", 0, false)
	if len(eps) != 0 {
		t.Errorf("latest (already downloaded): expected 0, got %d", len(eps))
	}
	delete(downloaded, "g5")

	eps, _ = selectEpisodesByDownloadPolicy(catalog, isDownloaded, "latest_k", 3, false)
	if len(eps) != 3 || eps[0].GUID != "g5" || eps[1].GUID != "g4" || eps[2].GUID != "g3" {
		t.Errorf("latest_3 (none downloaded): expected [g5, g4, g3], got %+v", eps)
	}

	downloaded["g5"] = true
	downloaded["g3"] = true
	eps, _ = selectEpisodesByDownloadPolicy(catalog, isDownloaded, "latest_k", 3, false)
	if len(eps) != 1 || eps[0].GUID != "g4" {
		t.Errorf("latest_3 (g5, g3 downloaded): expected [g4], got %+v", eps)
	}

	downloaded["g4"] = true
	eps, _ = selectEpisodesByDownloadPolicy(catalog, isDownloaded, "latest_k", 3, false)
	if len(eps) != 0 {
		t.Errorf("latest_3 (all 3 downloaded, older missing): expected 0, got %+v", eps)
	}

	eps, _ = selectEpisodesByDownloadPolicy(catalog, isDownloaded, "all", 0, false)
	if len(eps) != 2 || eps[0].GUID != "g2" || eps[1].GUID != "g1" {
		t.Errorf("all: expected [g2, g1], got %+v", eps)
	}

	downloaded["g1"] = true
	downloaded["g2"] = true
	eps, _ = selectEpisodesByDownloadPolicy(catalog, isDownloaded, "all", 0, false)
	if len(eps) != 0 {
		t.Errorf("all (all downloaded): expected 0, got %d", len(eps))
	}
}

func TestSelectEpisodesByDownloadPolicyEdgeCases(t *testing.T) {
	isDownloaded := func(ep FeedEpisode) bool { return false }

	eps, _ := selectEpisodesByDownloadPolicy(nil, isDownloaded, "latest", 0, false)
	if len(eps) != 0 {
		t.Errorf("empty catalog: expected 0, got %d", len(eps))
	}

	eps, _ = selectEpisodesByDownloadPolicy(nil, isDownloaded, "latest_k", 3, false)
	if len(eps) != 0 {
		t.Errorf("empty catalog latest_k: expected 0, got %d", len(eps))
	}

	eps, _ = selectEpisodesByDownloadPolicy(nil, isDownloaded, "all", 0, false)
	if len(eps) != 0 {
		t.Errorf("empty catalog all: expected 0, got %d", len(eps))
	}

	single := []FeedEpisode{
		{Title: "Ep 1", GUID: "g1", EnclosureURL: "http://example.com/1.mp3", PublishedAt: 1000},
	}
	eps, _ = selectEpisodesByDownloadPolicy(single, isDownloaded, "latest_k", 10, false)
	if len(eps) != 1 || eps[0].GUID != "g1" {
		t.Errorf("latest_10 on 1-item catalog: expected [g1], got %+v", eps)
	}

	catalog := []FeedEpisode{
		{Title: "Ep 1", GUID: "g1", EnclosureURL: "http://example.com/1.mp3", PublishedAt: 1000},
		{Title: "Ep 2", GUID: "g2", EnclosureURL: "http://example.com/2.mp3", PublishedAt: 2000},
	}
	epsOldest, _ := selectEpisodesByDownloadPolicy(catalog, isDownloaded, "latest_k", 2, true)
	if len(epsOldest) != 2 || epsOldest[0].GUID != "g1" || epsOldest[1].GUID != "g2" {
		t.Errorf("oldest latest_k: expected [g1, g2], got %+v", epsOldest)
	}
}
