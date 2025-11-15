package main

import (
	"context"
	"fmt"

	"github.com/wesnick/cmdg/pkg/cmdg"
)

// tokenInfoOutput is JSON output for token info
type tokenInfoOutput struct {
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	ExpiresIn     int64    `json:"expires_in"`
	Scope         string   `json:"scope"`
	Scopes        []string `json:"scopes"`
	UserID        string   `json:"user_id"`
	Audience      string   `json:"aud"`
	IssuedTo      string   `json:"issued_to"`
	AppName       string   `json:"app_name"`
	// Additional user profile information
	ProfileEmail         string `json:"profile_email"`
	MessagesTotal        int    `json:"messages_total"`
	ThreadsTotal         int    `json:"threads_total"`
	HistoryID            string `json:"history_id"`
}

// runAuthTokenInfo retrieves and displays information about the current OAuth token
func runAuthTokenInfo(ctx context.Context, conn *cmdg.CmdG, out *outputWriter) error {
	// Get token information
	out.writeVerbose("Fetching token information...")
	tokenInfo, err := conn.GetTokenInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token info: %w", err)
	}

	// Get user profile for additional information
	out.writeVerbose("Fetching user profile...")
	profile, err := conn.GetProfile(ctx)
	if err != nil {
		return fmt.Errorf("failed to get profile: %w", err)
	}

	if out.json {
		output := tokenInfoOutput{
			Email:         tokenInfo.Email,
			EmailVerified: tokenInfo.EmailVerified,
			ExpiresIn:     tokenInfo.ExpiresIn,
			Scope:         tokenInfo.Scope,
			Scopes:        tokenInfo.Scopes,
			UserID:        tokenInfo.UserID,
			Audience:      tokenInfo.Audience,
			IssuedTo:      tokenInfo.IssuedTo,
			AppName:       tokenInfo.AppName,
			ProfileEmail:  profile.EmailAddress,
			MessagesTotal: int(profile.MessagesTotal),
			ThreadsTotal:  int(profile.ThreadsTotal),
			HistoryID:     fmt.Sprintf("%d", profile.HistoryId),
		}
		return out.writeJSON(output)
	}

	// Text output
	out.writeMessage("=== OAuth Token Information ===")
	out.writeMessage("")

	out.writeMessage("User Information:")
	out.writeMessage(fmt.Sprintf("  Email:          %s", tokenInfo.Email))
	out.writeMessage(fmt.Sprintf("  Email Verified: %v", tokenInfo.EmailVerified))
	out.writeMessage(fmt.Sprintf("  User ID:        %s", tokenInfo.UserID))
	out.writeMessage("")

	out.writeMessage("Token Information:")
	out.writeMessage(fmt.Sprintf("  Expires In:     %d seconds", tokenInfo.ExpiresIn))
	out.writeMessage(fmt.Sprintf("  Audience:       %s", tokenInfo.Audience))
	out.writeMessage(fmt.Sprintf("  Issued To:      %s", tokenInfo.IssuedTo))
	if tokenInfo.AppName != "" {
		out.writeMessage(fmt.Sprintf("  App Name:       %s", tokenInfo.AppName))
	}
	out.writeMessage("")

	out.writeMessage("Granted Scopes:")
	for i, scope := range tokenInfo.Scopes {
		out.writeMessage(fmt.Sprintf("  %d. %s", i+1, scope))
	}
	out.writeMessage("")

	out.writeMessage("Gmail Profile:")
	out.writeMessage(fmt.Sprintf("  Email Address:  %s", profile.EmailAddress))
	out.writeMessage(fmt.Sprintf("  Total Messages: %d", profile.MessagesTotal))
	out.writeMessage(fmt.Sprintf("  Total Threads:  %d", profile.ThreadsTotal))
	out.writeMessage(fmt.Sprintf("  History ID:     %d", profile.HistoryId))

	return nil
}
