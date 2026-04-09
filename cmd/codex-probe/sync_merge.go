package main

import (
	"sort"
	"time"
)

type localSyncRecord struct {
	AccountID   string
	Path        string
	Token       *OAuthKey
	LastRefresh time.Time
}

type remoteSyncRecord struct {
	AccountID  string
	Email      string
	Token      *OAuthKey
	UpdateTime time.Time
}

type syncDecision struct {
	AccountID    string
	Token        *OAuthKey
	WriteLocal   bool
	UploadRemote bool
	Unchanged    bool
}

func mergeOAuthKeyPreferLocal(local, remote *OAuthKey, remoteEmail string) *OAuthKey {
	if local == nil {
		if remote == nil {
			return nil
		}
		merged := *remote
		if merged.Email == "" {
			merged.Email = remoteEmail
		}
		return &merged
	}

	merged := *local
	if remote == nil {
		return &merged
	}

	if merged.IDToken == "" {
		merged.IDToken = remote.IDToken
	}
	if merged.AccessToken == "" {
		merged.AccessToken = remote.AccessToken
	}
	if merged.RefreshToken == "" {
		merged.RefreshToken = remote.RefreshToken
	}
	if merged.AccountID == "" {
		merged.AccountID = remote.AccountID
	}
	if merged.LastRefresh == "" {
		merged.LastRefresh = remote.LastRefresh
	}
	if merged.Email == "" {
		merged.Email = remote.Email
	}
	if merged.Email == "" {
		merged.Email = remoteEmail
	}
	if merged.Type == "" {
		merged.Type = remote.Type
	}
	if merged.Expired == "" {
		merged.Expired = remote.Expired
	}

	return &merged
}

func mergeSyncRecords(local map[string]localSyncRecord, remote map[string]remoteSyncRecord) []syncDecision {
	keys := make(map[string]struct{}, len(local)+len(remote))
	for accountID := range local {
		keys[accountID] = struct{}{}
	}
	for accountID := range remote {
		keys[accountID] = struct{}{}
	}

	accountIDs := make([]string, 0, len(keys))
	for accountID := range keys {
		accountIDs = append(accountIDs, accountID)
	}
	sort.Strings(accountIDs)

	decisions := make([]syncDecision, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		localRec, hasLocal := local[accountID]
		remoteRec, hasRemote := remote[accountID]

		switch {
		case hasLocal && !hasRemote:
			decisions = append(decisions, syncDecision{
				AccountID:    accountID,
				Token:        localRec.Token,
				UploadRemote: true,
			})
		case !hasLocal && hasRemote:
			decisions = append(decisions, syncDecision{
				AccountID:  accountID,
				Token:      mergeOAuthKeyPreferLocal(nil, remoteRec.Token, remoteRec.Email),
				WriteLocal: true,
			})
		case hasLocal && hasRemote:
			if remoteRec.UpdateTime.After(localRec.LastRefresh) {
				decisions = append(decisions, syncDecision{
					AccountID:  accountID,
					Token:      mergeOAuthKeyPreferLocal(nil, remoteRec.Token, remoteRec.Email),
					WriteLocal: true,
				})
				continue
			}
			if !localRec.LastRefresh.Before(remoteRec.UpdateTime) && !remoteRec.UpdateTime.Before(localRec.LastRefresh) {
				decisions = append(decisions, syncDecision{
					AccountID: accountID,
					Token:     mergeOAuthKeyPreferLocal(localRec.Token, remoteRec.Token, remoteRec.Email),
					Unchanged: true,
				})
				continue
			}
			decisions = append(decisions, syncDecision{
				AccountID:    accountID,
				Token:        mergeOAuthKeyPreferLocal(localRec.Token, remoteRec.Token, remoteRec.Email),
				UploadRemote: true,
			})
		}
	}

	return decisions
}
