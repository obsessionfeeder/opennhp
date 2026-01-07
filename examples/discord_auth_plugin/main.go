package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/OpenNHP/opennhp/nhp/common"
	nhplog "github.com/OpenNHP/opennhp/nhp/log"
	"github.com/OpenNHP/opennhp/nhp/plugins"
	"github.com/OpenNHP/opennhp/nhp/utils"
	"github.com/gin-gonic/gin"

	_ "github.com/mattn/go-sqlite3"
	toml "github.com/pelletier/go-toml/v2"
)

type config struct {
	KeysDbPath string
	BffApiUrl  string // BFF API URL for subscription validation
}

var (
	log           *nhplog.Logger
	pluginDirPath string
	hostname      string
	localIp       string
	localMac      string
)

var (
	name    = "discord"
	version = "0.3.1" // Skip AC for subscription-based downloads (nginx auth_request only)

	baseConfigWatch io.Closer
	resConfigWatch  io.Closer

	baseConf         *config
	resourceMapMutex sync.Mutex
	resourceMap      common.ResourceGroupMap

	db      *sql.DB
	dbMutex sync.Mutex
)

var (
	errLoadConfig = fmt.Errorf("config load error")
)

func Version() string {
	return fmt.Sprintf("%s v%s", name, version)
}

func Signature() string {
	return "discord-auth-plugin"
}

func ExportedData() *plugins.PluginParamsOut {
	return nil
}

func Init(in *plugins.PluginParamsIn) error {
	if in.PluginDirPath != nil {
		pluginDirPath = *in.PluginDirPath
	}
	if in.Log != nil {
		log = in.Log
	}
	if in.Hostname != nil {
		hostname = *in.Hostname
	}
	if in.LocalIp != nil {
		localIp = *in.LocalIp
	}
	if in.LocalMac != nil {
		localMac = *in.LocalMac
	}

	// load config
	fileNameBase := filepath.Join(pluginDirPath, "etc", "config.toml")
	if err := updateConfig(fileNameBase); err != nil {
		_ = err
	}

	baseConfigWatch = utils.WatchFile(fileNameBase, func() {
		log.Info("base config: %s has been updated", fileNameBase)
		updateConfig(fileNameBase)
	})

	fileNameRes := filepath.Join(pluginDirPath, "etc", "resource.toml")
	if err := updateResource(fileNameRes); err != nil {
		_ = err
	}
	resConfigWatch = utils.WatchFile(fileNameRes, func() {
		log.Info("resource config: %s has been updated", fileNameRes)
		updateResource(fileNameRes)
	})

	// Initialize keys database
	if err := initKeysDb(); err != nil {
		log.Error("failed to initialize keys database: %v", err)
	}

	return nil
}

func initKeysDb() error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	if baseConf == nil || baseConf.KeysDbPath == "" {
		return fmt.Errorf("keys database path not configured")
	}

	var err error
	dbPath := baseConf.KeysDbPath
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(pluginDirPath, dbPath)
	}

	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	// Create table if not exists (schema matches Node.js Discord bot)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS nhp_keys (
			id TEXT PRIMARY KEY,
			discord_id TEXT UNIQUE NOT NULL,
			discord_username TEXT NOT NULL,
			nhp_key TEXT UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			last_used_at DATETIME,
			revoked BOOLEAN DEFAULT 0,
			revoked_at DATETIME,
			revoked_reason TEXT
		)
	`)
	if err != nil {
		return err
	}

	// Create IP whitelist table for true invisibility
	// nginx auth_request will check this table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS nhp_ip_whitelist (
			ip TEXT PRIMARY KEY,
			discord_id TEXT NOT NULL,
			discord_username TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL
		)
	`)
	if err != nil {
		log.Error("Failed to create IP whitelist table: %v", err)
		return err
	}

	// Create index for efficient expiry cleanup
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_ip_whitelist_expires ON nhp_ip_whitelist(expires_at)`)

	// Clean up expired entries on startup
	_, _ = db.Exec(`DELETE FROM nhp_ip_whitelist WHERE expires_at < datetime('now')`)

	log.Info("Keys database initialized: %s", dbPath)
	return nil
}

func updateConfig(file string) (err error) {
	utils.CatchPanicThenRun(func() {
		err = errLoadConfig
	})

	content, err := os.ReadFile(file)
	if err != nil {
		log.Error("failed to read base config: %v", err)
		return err
	}

	var conf config
	if err := toml.Unmarshal(content, &conf); err != nil {
		log.Error("failed to unmarshal base config: %v", err)
		return err
	}

	baseConf = &conf
	return nil
}

func updateResource(file string) (err error) {
	utils.CatchPanicThenRun(func() {
		err = errLoadConfig
	})

	content, err := os.ReadFile(file)
	if err != nil {
		log.Error("failed to read resource config: %v", err)
		return err
	}

	resourceMapMutex.Lock()
	defer resourceMapMutex.Unlock()

	resourceMap = make(common.ResourceGroupMap)
	if err := toml.Unmarshal(content, &resourceMap); err != nil {
		log.Error("failed to unmarshal resource config: %v", err)
		return err
	}

	for resId, res := range resourceMap {
		res.AuthServiceId = name
		res.ResourceId = resId
	}

	return nil
}

func Close() error {
	if baseConfigWatch != nil {
		baseConfigWatch.Close()
	}
	if resConfigWatch != nil {
		resConfigWatch.Close()
	}
	if db != nil {
		db.Close()
	}
	return nil
}

func findResource(resId string) *common.ResourceData {
	resourceMapMutex.Lock()
	defer resourceMapMutex.Unlock()

	res, found := resourceMap[resId]
	if found {
		return res
	}
	return nil
}

// validateNhpKey checks if the NHP key is valid and not revoked
func validateNhpKey(nhpKey string) (discordId string, valid bool) {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	if db == nil {
		log.Error("database not initialized")
		return "", false
	}

	var revoked bool
	err := db.QueryRow(
		"SELECT discord_id, revoked FROM nhp_keys WHERE nhp_key = ?",
		nhpKey,
	).Scan(&discordId, &revoked)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Info("NHP key not found: %s", nhpKey)
		} else {
			log.Error("database query error: %v", err)
		}
		return "", false
	}

	if revoked {
		log.Info("NHP key has been revoked: %s", nhpKey)
		return "", false
	}

	// Update last used timestamp
	_, _ = db.Exec(
		"UPDATE nhp_keys SET last_used_at = ? WHERE nhp_key = ?",
		time.Now(), nhpKey,
	)

	return discordId, true
}

// validateAndConsumeInviteToken validates a one-time invitation token
// Returns the Discord ID and username if valid, empty strings otherwise
// The token is marked as used after successful validation
func validateAndConsumeInviteToken(token string, sourceIP string) (discordId string, discordUsername string, valid bool) {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	if db == nil {
		log.Error("database not initialized")
		return "", "", false
	}

	// Check token exists and is valid (join with nhp_keys to get username)
	var nhpKeyId string
	var expiresAt string
	var used bool

	err := db.QueryRow(`
		SELECT t.nhp_key_id, t.discord_id, t.expires_at, t.used, k.discord_username
		FROM nhp_invite_tokens t
		JOIN nhp_keys k ON t.nhp_key_id = k.id
		WHERE t.token = ?`,
		token,
	).Scan(&nhpKeyId, &discordId, &expiresAt, &used, &discordUsername)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Info("Invite token not found: %s...", token[:min(8, len(token))])
		} else {
			log.Error("database query error: %v", err)
		}
		return "", "", false
	}

	// Check if already used
	if used {
		log.Info("Invite token already used: %s...", token[:min(8, len(token))])
		return "", "", false
	}

	// Check if expired
	expires, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		// Try alternate format
		expires, err = time.Parse("2006-01-02 15:04:05", expiresAt)
	}
	if err != nil {
		log.Error("Failed to parse expires_at: %v", err)
		return "", "", false
	}

	if time.Now().After(expires) {
		log.Info("Invite token expired: %s...", token[:min(8, len(token))])
		return "", "", false
	}

	// Check if the associated NHP key is still valid
	var keyRevoked bool
	err = db.QueryRow(
		"SELECT revoked FROM nhp_keys WHERE id = ?",
		nhpKeyId,
	).Scan(&keyRevoked)

	if err != nil || keyRevoked {
		log.Info("Invite token's NHP key is revoked: %s...", token[:min(8, len(token))])
		return "", "", false
	}

	// Mark token as used (one-time use)
	_, err = db.Exec(
		"UPDATE nhp_invite_tokens SET used = 1, used_at = ?, used_ip = ? WHERE token = ?",
		time.Now().Format(time.RFC3339), sourceIP, token,
	)
	if err != nil {
		log.Error("Failed to mark token as used: %v", err)
		// Continue anyway - token was valid
	}

	log.Info("Invite token consumed: %s... from IP %s (user: %s)", token[:min(8, len(token))], sourceIP, discordUsername)
	return discordId, discordUsername, true
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// addToIpWhitelist adds an IP to the whitelist with expiry
// This is called after a successful knock to allow the IP through nginx auth_request
func addToIpWhitelist(ip string, discordId string, discordUsername string, openTimeSeconds int) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	expiresAt := time.Now().Add(time.Duration(openTimeSeconds) * time.Second)

	// Upsert - replace if IP already exists (user re-knocking)
	_, err := db.Exec(`
		INSERT INTO nhp_ip_whitelist (ip, discord_id, discord_username, created_at, expires_at)
		VALUES (?, ?, ?, datetime('now'), ?)
		ON CONFLICT(ip) DO UPDATE SET
			discord_id = excluded.discord_id,
			discord_username = excluded.discord_username,
			created_at = datetime('now'),
			expires_at = excluded.expires_at
	`, ip, discordId, discordUsername, expiresAt.Format(time.RFC3339))

	if err != nil {
		log.Error("Failed to add IP to whitelist: %v", err)
		return err
	}

	log.Info("IP %s whitelisted for Discord user %s (%s) until %s", ip, discordUsername, discordId, expiresAt.Format(time.RFC3339))
	return nil
}

// isIpWhitelisted checks if an IP is in the whitelist and not expired
func isIpWhitelisted(ip string) (discordId string, discordUsername string, whitelisted bool) {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	if db == nil {
		log.Error("database not initialized")
		return "", "", false
	}

	// Clean up expired entries first (periodically)
	_, _ = db.Exec(`DELETE FROM nhp_ip_whitelist WHERE expires_at < datetime('now')`)

	var expiresAtStr string
	var usernameNull sql.NullString
	err := db.QueryRow(
		"SELECT discord_id, discord_username, expires_at FROM nhp_ip_whitelist WHERE ip = ?",
		ip,
	).Scan(&discordId, &usernameNull, &expiresAtStr)

	if err != nil {
		if err == sql.ErrNoRows {
			// Not whitelisted - this is expected for most requests
			return "", "", false
		}
		log.Error("database query error checking IP whitelist: %v", err)
		return "", "", false
	}

	if usernameNull.Valid {
		discordUsername = usernameNull.String
	}

	// Parse expiry time
	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		// Try alternate format
		expiresAt, err = time.Parse("2006-01-02 15:04:05", expiresAtStr)
	}
	if err != nil {
		log.Error("Failed to parse expires_at for IP %s: %v", ip, err)
		return "", "", false
	}

	if time.Now().After(expiresAt) {
		// Expired
		return "", "", false
	}

	return discordId, discordUsername, true
}

func RequestOTP(req *common.NhpOTPRequest, helper *plugins.NhpServerPluginHelper) error {
	return fmt.Errorf("OTP not implemented for discord auth")
}

func RegisterAgent(req *common.NhpRegisterRequest, helper *plugins.NhpServerPluginHelper) (*common.ServerRegisterAckMsg, error) {
	return nil, fmt.Errorf("RegisterAgent not implemented for discord auth")
}

func ListService(req *common.NhpListRequest, helper *plugins.NhpServerPluginHelper) (*common.ServerListResultMsg, error) {
	return nil, fmt.Errorf("ListService not implemented for discord auth")
}

func AuthWithHttp(ctx *gin.Context, req *common.HttpKnockRequest, helper *plugins.HttpServerPluginHelper) (ackMsg *common.ServerKnockAckMsg, err error) {
	if helper == nil {
		return nil, fmt.Errorf("AuthWithHttp: helper is null")
	}

	resId := ctx.Query("resid")
	action := ctx.Query("action")
	if len(resId) > 0 && strings.Contains(resId, "|") {
		params := strings.Split(resId, "|")
		resId = params[0]
		if len(params) > 1 {
			action = params[1]
		}
	}

	res := findResource(resId)
	if res == nil || len(res.Resources) == 0 {
		ackMsg = nil
		err = common.ErrResourceNotFound
		log.Error("resource error: %v", err)
		ctx.String(http.StatusOK, "{\"errMsg\": \"resource error: %v\"}", err)
		return
	}

	corsMiddleware(ctx)

	switch {
	case strings.EqualFold(action, "knock"):
		// One-time knock via invitation token (invisible access)
		ackMsg, err = knockWithInviteToken(ctx, req, res, helper)

	case strings.EqualFold(action, "download-knock"):
		// Subscription-based knock for downloads (mobile app)
		ackMsg, err = knockWithSubscription(ctx, req, res, helper)

	case strings.EqualFold(action, "check-ip"):
		// IP whitelist check for nginx auth_request
		// Returns 200 if IP is whitelisted, 403 if not
		ackMsg, err = checkIpWhitelist(ctx)

	case strings.EqualFold(action, "valid"):
		ackMsg, err = authWithNhpKey(ctx, req, res, helper)

	case strings.EqualFold(action, "login"):
		// DEPRECATED: Login page removed for true invisibility
		// Return 444 (nginx: close connection without response)
		ctx.AbortWithStatus(444)
		return nil, nil

	default:
		// Silent fail for invalid actions - don't reveal anything
		ctx.AbortWithStatus(444)
		return nil, nil
	}
	return
}

func showLoginPage(ctx *gin.Context, req *common.HttpKnockRequest, res *common.ResourceData, helper *plugins.HttpServerPluginHelper) (*common.ServerKnockAckMsg, error) {
	_ = helper

	title := "Cottonwood Access"
	if res.ExInfo != nil {
		if t, ok := res.ExInfo["Title"].(string); ok {
			title = t
		}
	}

	ctx.HTML(http.StatusOK, "discord-auth/login.html", gin.H{
		"title":     title,
		"nhpServer": hostname,
		"aspId":     req.AuthServiceId,
		"resId":     res.ResourceId,
	})

	return nil, nil
}

// checkIpWhitelist handles nginx auth_request for IP-based access control
// Returns 200 if IP is whitelisted (with X-NHP-Discord-ID and X-NHP-Discord-Username headers)
// Returns 403 if not whitelisted (nginx should convert this to 444)
func checkIpWhitelist(ctx *gin.Context) (*common.ServerKnockAckMsg, error) {
	// Get client IP - try X-Real-IP first, then X-Forwarded-For, then RemoteAddr
	ip := ctx.GetHeader("X-Real-IP")
	if ip == "" {
		ip = ctx.GetHeader("X-Forwarded-For")
		if ip != "" {
			// X-Forwarded-For can contain multiple IPs, take the first
			parts := strings.Split(ip, ",")
			ip = strings.TrimSpace(parts[0])
		}
	}
	if ip == "" {
		ip = ctx.ClientIP()
	}

	discordId, discordUsername, whitelisted := isIpWhitelisted(ip)
	if whitelisted {
		// Set Discord ID and username headers for upstream use
		ctx.Header("X-NHP-Discord-ID", discordId)
		if discordUsername != "" {
			ctx.Header("X-NHP-Discord-Username", discordUsername)
		}
		ctx.Status(http.StatusOK)
		return nil, nil
	}

	// Not whitelisted - nginx will convert 403 to 444 (silent close)
	ctx.Status(http.StatusForbidden)
	return nil, nil
}

// knockWithInviteToken handles one-time knock via invitation token
// This is the primary method for invisible access - no login page needed
func knockWithInviteToken(ctx *gin.Context, req *common.HttpKnockRequest, res *common.ResourceData, helper *plugins.HttpServerPluginHelper) (*common.ServerKnockAckMsg, error) {
	// Get token from query parameter
	token, _ := url.QueryUnescape(ctx.Query("t"))
	if token == "" {
		token, _ = url.QueryUnescape(ctx.Query("token"))
	}

	if token == "" {
		// Silent fail - don't reveal anything
		log.Info("No invite token provided")
		ctx.AbortWithStatus(444)
		return nil, nil
	}

	// Get source IP for logging and potential IP-based access control
	sourceIP := ctx.ClientIP()

	// Validate and consume the token (one-time use)
	discordId, discordUsername, valid := validateAndConsumeInviteToken(token, sourceIP)
	if !valid {
		// Silent fail - don't reveal anything
		ctx.AbortWithStatus(444)
		return nil, nil
	}

	req.UserId = discordId

	// Call AC to open access (triggers iptables rule for this IP)
	ackMsg, err := helper.AuthWithHttpCallbackFunc(req, res)
	if ackMsg == nil || ackMsg.ErrCode != common.ErrSuccess.ErrorCode() {
		log.Error("knock failed for Discord user: %s (%s)", discordUsername, discordId)
		// Silent fail
		ctx.AbortWithStatus(444)
		return nil, err
	}

	log.Info("Knock succeeded for Discord user: %s (%s) from IP %s", discordUsername, discordId, sourceIP)

	// Add IP to whitelist for nginx auth_request (true invisibility)
	// Use res.OpenTime as the whitelist duration
	if err := addToIpWhitelist(sourceIP, discordId, discordUsername, int(res.OpenTime)); err != nil {
		log.Error("Failed to add IP to whitelist: %v", err)
		// Continue anyway - AC already opened access
	}

	// Set cookies (still needed for WebSocket auth within the session)
	singleHost := len(ackMsg.ACTokens) == 1
	for resName, acToken := range ackMsg.ACTokens {
		if singleHost {
			ctx.SetCookie(
				"nhp-token",
				url.QueryEscape(acToken),
				int(res.OpenTime),
				"/",
				res.CookieDomain,
				true, // Secure - required for HTTPS
				true, // HttpOnly
			)
		} else {
			domain := strings.Split(ackMsg.ResourceHost[resName], ":")[0]
			ctx.SetCookie(
				"nhp-token/"+resName,
				url.QueryEscape(acToken),
				int(res.OpenTime),
				"/",
				domain,
				true,
				true,
			)
		}
	}

	// Set Discord ID cookie for Cottonwood
	ctx.SetCookie(
		"nhp-discord-id",
		discordId,
		int(res.OpenTime),
		"/",
		res.CookieDomain,
		true,  // Secure
		false, // Allow JavaScript access
	)

	// Redirect to the app (now accessible because IP is whitelisted by AC)
	if len(res.RedirectUrl) > 0 {
		ctx.Redirect(http.StatusFound, res.RedirectUrl)
	} else {
		log.Error("RedirectUrl is not provided")
		ctx.Redirect(http.StatusFound, "/")
	}

	return ackMsg, nil
}

// authWithNhpKey is DEPRECATED - kept for backwards compatibility but returns 444
func authWithNhpKey(ctx *gin.Context, req *common.HttpKnockRequest, res *common.ResourceData, helper *plugins.HttpServerPluginHelper) (*common.ServerKnockAckMsg, error) {
	// DEPRECATED: Direct NHP key auth is disabled for true invisibility
	// Users should use invitation tokens via /knock endpoint
	log.Info("Direct NHP key auth attempted - deprecated, returning 444")
	ctx.AbortWithStatus(444)
	return nil, nil
}

func AuthWithNHP(req *common.NhpAuthRequest, helper *plugins.NhpServerPluginHelper) (ackMsg *common.ServerKnockAckMsg, err error) {
	ackMsg = req.Ack
	if helper == nil {
		return ackMsg, fmt.Errorf("AuthWithNHP: helper is null")
	}

	var found bool
	var res *common.ResourceData
	resourceMapMutex.Lock()
	res, found = resourceMap[req.Msg.ResourceId]
	resourceMapMutex.Unlock()

	if !found || len(res.Resources) == 0 {
		err = common.ErrResourceNotFound
		ackMsg.ErrCode = common.ErrResourceNotFound.ErrorCode()
		ackMsg.ErrMsg = err.Error()
		return
	}

	// For NHP protocol auth, the NHP key is passed in UserId field
	// The Discord bot sets UserId to the NHP key when knocking
	nhpKey := req.Msg.UserId
	if nhpKey == "" {
		err = common.ErrBackendAuthRequired
		ackMsg.ErrCode = common.ErrBackendAuthRequired.ErrorCode()
		ackMsg.ErrMsg = "NHP key required"
		return
	}

	discordId, valid := validateNhpKey(nhpKey)
	if !valid {
		err = common.ErrBackendAuthRequired
		ackMsg.ErrCode = common.ErrBackendAuthRequired.ErrorCode()
		ackMsg.ErrMsg = "Invalid or revoked NHP key"
		return
	}

	log.Info("NHP auth succeeded for Discord user: %s", discordId)
	ackMsg.OpenTime = res.OpenTime

	ackMsg, err = helper.AuthWithNhpCallbackFunc(req, res)
	return ackMsg, err
}

func corsMiddleware(ctx *gin.Context) {
	originResource := ctx.Request.Header.Get("Origin")

	if originResource != "" {
		ctx.Writer.Header().Set("Access-Control-Allow-Origin", originResource)
	}

	ctx.Next()
}

// knockWithSubscription handles subscription-based knock for mobile app downloads
// This validates the user's premium subscription via BFF/RevenueCat before whitelisting
func knockWithSubscription(ctx *gin.Context, req *common.HttpKnockRequest, res *common.ResourceData, helper *plugins.HttpServerPluginHelper) (*common.ServerKnockAckMsg, error) {
	// Get device ID from header (sent by Flutter app)
	deviceId := ctx.GetHeader("X-Device-ID")
	if deviceId == "" {
		deviceId = ctx.Query("device_id")
	}

	if deviceId == "" {
		log.Info("No device ID provided for download knock")
		ctx.AbortWithStatus(444)
		return nil, nil
	}

	// Get API key from header (validates it's a legitimate app request)
	apiKey := ctx.GetHeader("X-API-Key")
	if apiKey == "" {
		log.Info("No API key provided for download knock")
		ctx.AbortWithStatus(444)
		return nil, nil
	}

	// Get source IP
	sourceIP := ctx.ClientIP()

	// Validate subscription via BFF
	isPremium, err := validateSubscriptionViaBFF(deviceId, apiKey)
	if err != nil {
		log.Error("Subscription validation error: %v", err)
		ctx.AbortWithStatus(444)
		return nil, nil
	}

	if !isPremium {
		log.Info("Non-premium user attempted download knock: device=%s", deviceId)
		// Return 403 for non-premium - app can show upgrade prompt
		ctx.JSON(http.StatusForbidden, gin.H{
			"error":   "subscription_required",
			"message": "Premium subscription required for offline downloads",
		})
		return nil, nil
	}

	req.UserId = deviceId

	// For downloads with RequiresSubscription, we skip the AC call
	// (downloads use nginx auth_request + IP whitelist, not iptables)
	// The subscription check above is sufficient authorization
	var ackMsg *common.ServerKnockAckMsg
	requiresSubscription := false
	if res.ExInfo != nil {
		if rs, ok := res.ExInfo["RequiresSubscription"].(bool); ok {
			requiresSubscription = rs
		}
	}

	if !requiresSubscription {
		// Call AC to open access (triggers iptables rule for this IP)
		var err error
		ackMsg, err = helper.AuthWithHttpCallbackFunc(req, res)
		if err != nil {
			log.Error("AuthWithHttpCallbackFunc failed: %v", err)
			ctx.AbortWithStatus(444)
			return nil, err
		}

		if ackMsg == nil || ackMsg.ErrCode != common.ErrSuccess.ErrorCode() {
			log.Error("knock failed for device: %s", deviceId)
			ctx.AbortWithStatus(444)
			return nil, err
		}
	}

	log.Info("Download knock succeeded for device: %s from IP %s (skipAC=%v)", deviceId, sourceIP, requiresSubscription)

	// Add IP to whitelist for nginx auth_request
	// Use res.OpenTime as the whitelist duration (default 1 hour for downloads)
	if err := addToIpWhitelist(sourceIP, deviceId, "", int(res.OpenTime)); err != nil {
		log.Error("Failed to add IP to whitelist: %v", err)
		// Continue anyway - AC already opened access
	}

	// Return success response (no redirect needed - mobile app handles download)
	ctx.JSON(http.StatusOK, gin.H{
		"success":    true,
		"open_time":  res.OpenTime,
		"message":    "IP whitelisted for downloads",
		"expires_in": res.OpenTime,
	})

	return ackMsg, nil
}

// validateSubscriptionViaBFF calls the BFF API to check if device has premium subscription
func validateSubscriptionViaBFF(deviceId string, apiKey string) (bool, error) {
	if baseConf == nil || baseConf.BffApiUrl == "" {
		return false, fmt.Errorf("BFF API URL not configured")
	}

	// Build request to BFF subscription validation endpoint
	reqUrl := fmt.Sprintf("%s/api/v1/subscription/validate", baseConf.BffApiUrl)

	httpReq, err := http.NewRequest("POST", reqUrl, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-Device-ID", deviceId)
	httpReq.Header.Set("X-API-Key", apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("BFF request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("BFF returned status %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		IsPremium    bool   `json:"is_premium"`
		Entitlements any    `json:"entitlements"`
		ExpiresAt    string `json:"expires_at"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	// Simple JSON parsing without encoding/json import (already using gin)
	// Use gin's built-in JSON binding
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Info("Subscription check for device %s: premium=%v, expires=%s", deviceId, result.IsPremium, result.ExpiresAt)

	return result.IsPremium, nil
}

func main() {
	// Plugin main - not used directly
}
