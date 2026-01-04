package main

import (
	"database/sql"
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
	version = "0.1.0"

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
	case strings.EqualFold(action, "valid"):
		ackMsg, err = authWithNhpKey(ctx, req, res, helper)

	case strings.EqualFold(action, "login"):
		ackMsg, err = showLoginPage(ctx, req, res, helper)

	default:
		ackMsg = nil
		err = fmt.Errorf("action invalid")
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

func authWithNhpKey(ctx *gin.Context, req *common.HttpKnockRequest, res *common.ResourceData, helper *plugins.HttpServerPluginHelper) (*common.ServerKnockAckMsg, error) {
	nhpKey, _ := url.QueryUnescape(ctx.Query("nhp_key"))
	if nhpKey == "" {
		nhpKey, _ = url.QueryUnescape(ctx.Query("nhpKey"))
	}

	if nhpKey == "" {
		log.Info("No NHP key provided")
		ctx.JSON(http.StatusOK, gin.H{
			"errCode": 400,
			"errMsg":  "NHP key is required",
		})
		return nil, fmt.Errorf("NHP key is required")
	}

	discordId, valid := validateNhpKey(nhpKey)
	if !valid {
		log.Info("Invalid or revoked NHP key: %s", nhpKey)
		ctx.JSON(http.StatusOK, gin.H{
			"errCode": 401,
			"errMsg":  "Invalid or revoked NHP key. Please get a new key from Discord using /nhp-access",
		})
		return nil, fmt.Errorf("invalid NHP key")
	}

	req.UserId = discordId

	// Call AC to open access
	ackMsg, err := helper.AuthWithHttpCallbackFunc(req, res)
	if ackMsg == nil || ackMsg.ErrCode != common.ErrSuccess.ErrorCode() {
		log.Error("knock failed")
		ackMsg = &common.ServerKnockAckMsg{}
		ackMsg.ErrCode = common.ErrServerACOpsFailed.ErrorCode()
		if err != nil {
			ackMsg.ErrMsg = err.Error()
		} else {
			ackMsg.ErrMsg = "Failed to open access"
		}
		ctx.JSON(http.StatusOK, ackMsg)
		return ackMsg, err
	}

	log.Info("knock succeeded for Discord user: %s", discordId)
	ackMsg.ErrMsg = ""

	// Set redirect URL
	if len(res.RedirectUrl) == 0 {
		log.Error("RedirectUrl is not provided")
	} else {
		ackMsg.RedirectUrl = res.RedirectUrl
	}

	// Set cookies
	singleHost := len(ackMsg.ACTokens) == 1
	for resName, token := range ackMsg.ACTokens {
		if singleHost {
			ctx.SetCookie(
				"nhp-token",
				url.QueryEscape(token),
				int(res.OpenTime),
				"/",
				res.CookieDomain,
				true, // Secure - required for HTTPS
				true,
			)
		} else {
			domain := strings.Split(ackMsg.ResourceHost[resName], ":")[0]
			ctx.SetCookie(
				"nhp-token/"+resName,
				url.QueryEscape(token),
				int(res.OpenTime),
				"/",
				domain,
				true, // Secure - required for HTTPS
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
		true, // Secure - required for HTTPS
		false, // Allow JavaScript access
	)

	ctx.JSON(http.StatusOK, ackMsg)
	return ackMsg, nil
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

func main() {
	// Plugin main - not used directly
}
