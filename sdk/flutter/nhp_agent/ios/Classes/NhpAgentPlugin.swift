import Flutter
import UIKit

// Import the NhpAgent framework (compiled from Go using gomobile)
// This will be available after running scripts/build_native.sh
#if canImport(NhpAgent)
import NhpAgent
#endif

public class NhpAgentPlugin: NSObject, FlutterPlugin {
    public static func register(with registrar: FlutterPluginRegistrar) {
        let channel = FlutterMethodChannel(
            name: "io.opennhp/nhp_agent",
            binaryMessenger: registrar.messenger()
        )
        let instance = NhpAgentPlugin()
        registrar.addMethodCallDelegate(instance, channel: channel)
    }

    public func handle(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
        guard let args = call.arguments as? [String: Any] else {
            if call.method == "close" || call.method == "stopKnockLoop" {
                // These methods don't require arguments
            } else {
                result(FlutterError(code: "INVALID_ARGS", message: "Arguments required", details: nil))
                return
            }
        }

        switch call.method {
        case "initialize":
            handleInitialize(args: call.arguments as? [String: Any] ?? [:], result: result)

        case "close":
            handleClose(result: result)

        case "setKnockUser":
            handleSetKnockUser(args: call.arguments as? [String: Any] ?? [:], result: result)

        case "addServer":
            handleAddServer(args: call.arguments as? [String: Any] ?? [:], result: result)

        case "removeServer":
            handleRemoveServer(args: call.arguments as? [String: Any] ?? [:], result: result)

        case "addResource":
            handleAddResource(args: call.arguments as? [String: Any] ?? [:], result: result)

        case "removeResource":
            handleRemoveResource(args: call.arguments as? [String: Any] ?? [:], result: result)

        case "knock":
            handleKnock(args: call.arguments as? [String: Any] ?? [:], result: result)

        case "exitResource":
            handleExitResource(args: call.arguments as? [String: Any] ?? [:], result: result)

        case "startKnockLoop":
            handleStartKnockLoop(result: result)

        case "stopKnockLoop":
            handleStopKnockLoop(result: result)

        case "generateKeys":
            handleGenerateKeys(args: call.arguments as? [String: Any] ?? [:], result: result)

        case "privateKeyToPublicKey":
            handlePrivateKeyToPublicKey(args: call.arguments as? [String: Any] ?? [:], result: result)

        default:
            result(FlutterMethodNotImplemented)
        }
    }

    // MARK: - Method Handlers

    private func handleInitialize(args: [String: Any], result: @escaping FlutterResult) {
        #if canImport(NhpAgent)
        guard let workingDir = args["workingDir"] as? String else {
            result(FlutterError(code: "INVALID_ARGS", message: "workingDir is required", details: nil))
            return
        }
        let logLevel = args["logLevel"] as? Int ?? 2

        let success = IossdkNhpAgentInit(workingDir, logLevel)
        result(success)
        #else
        result(FlutterError(code: "NOT_COMPILED", message: "NhpAgent framework not available. Run scripts/build_native.sh first.", details: nil))
        #endif
    }

    private func handleClose(result: @escaping FlutterResult) {
        #if canImport(NhpAgent)
        IossdkNhpAgentClose()
        result(nil)
        #else
        result(nil)
        #endif
    }

    private func handleSetKnockUser(args: [String: Any], result: @escaping FlutterResult) {
        #if canImport(NhpAgent)
        let userId = args["userId"] as? String ?? ""
        let deviceId = args["deviceId"] as? String ?? ""
        let orgId = args["orgId"] as? String ?? ""
        let userData = args["userData"] as? String ?? ""

        let success = IossdkNhpAgentSetKnockUser(userId, deviceId, orgId, userData)
        result(success)
        #else
        result(FlutterError(code: "NOT_COMPILED", message: "NhpAgent framework not available", details: nil))
        #endif
    }

    private func handleAddServer(args: [String: Any], result: @escaping FlutterResult) {
        #if canImport(NhpAgent)
        guard let pubkey = args["pubkey"] as? String else {
            result(FlutterError(code: "INVALID_ARGS", message: "pubkey is required", details: nil))
            return
        }
        let ip = args["ip"] as? String ?? ""
        let host = args["host"] as? String ?? ""
        let port = args["port"] as? Int ?? 62206
        let expireTime = args["expireTime"] as? Int64 ?? 0

        let success = IossdkNhpAgentAddServer(pubkey, ip, host, port, expireTime)
        result(success)
        #else
        result(FlutterError(code: "NOT_COMPILED", message: "NhpAgent framework not available", details: nil))
        #endif
    }

    private func handleRemoveServer(args: [String: Any], result: @escaping FlutterResult) {
        #if canImport(NhpAgent)
        let pubkey = args["pubkey"] as? String ?? ""
        IossdkNhpAgentRemoveServer(pubkey)
        result(nil)
        #else
        result(nil)
        #endif
    }

    private func handleAddResource(args: [String: Any], result: @escaping FlutterResult) {
        #if canImport(NhpAgent)
        guard let authServiceId = args["authServiceId"] as? String,
              let resourceId = args["resourceId"] as? String else {
            result(FlutterError(code: "INVALID_ARGS", message: "authServiceId and resourceId are required", details: nil))
            return
        }
        let serverIp = args["serverIp"] as? String ?? ""
        let serverHost = args["serverHost"] as? String ?? ""
        let serverPort = args["serverPort"] as? Int ?? 62206

        let success = IossdkNhpAgentAddResource(authServiceId, resourceId, serverIp, serverHost, serverPort)
        result(success)
        #else
        result(FlutterError(code: "NOT_COMPILED", message: "NhpAgent framework not available", details: nil))
        #endif
    }

    private func handleRemoveResource(args: [String: Any], result: @escaping FlutterResult) {
        #if canImport(NhpAgent)
        let authServiceId = args["authServiceId"] as? String ?? ""
        let resourceId = args["resourceId"] as? String ?? ""
        IossdkNhpAgentRemoveResource(authServiceId, resourceId)
        result(nil)
        #else
        result(nil)
        #endif
    }

    private func handleKnock(args: [String: Any], result: @escaping FlutterResult) {
        #if canImport(NhpAgent)
        guard let authServiceId = args["authServiceId"] as? String,
              let resourceId = args["resourceId"] as? String else {
            result(FlutterError(code: "INVALID_ARGS", message: "authServiceId and resourceId are required", details: nil))
            return
        }
        let serverIp = args["serverIp"] as? String ?? ""
        let serverHost = args["serverHost"] as? String ?? ""
        let serverPort = args["serverPort"] as? Int ?? 62206

        // Perform knock on background thread to avoid blocking UI
        DispatchQueue.global(qos: .userInitiated).async {
            let jsonResult = IossdkNhpAgentKnockResource(authServiceId, resourceId, serverIp, serverHost, serverPort)
            DispatchQueue.main.async {
                result(jsonResult)
            }
        }
        #else
        result(FlutterError(code: "NOT_COMPILED", message: "NhpAgent framework not available", details: nil))
        #endif
    }

    private func handleExitResource(args: [String: Any], result: @escaping FlutterResult) {
        #if canImport(NhpAgent)
        guard let authServiceId = args["authServiceId"] as? String,
              let resourceId = args["resourceId"] as? String else {
            result(FlutterError(code: "INVALID_ARGS", message: "authServiceId and resourceId are required", details: nil))
            return
        }
        let serverIp = args["serverIp"] as? String ?? ""
        let serverHost = args["serverHost"] as? String ?? ""
        let serverPort = args["serverPort"] as? Int ?? 62206

        let success = IossdkNhpAgentExitResource(authServiceId, resourceId, serverIp, serverHost, serverPort)
        result(success)
        #else
        result(FlutterError(code: "NOT_COMPILED", message: "NhpAgent framework not available", details: nil))
        #endif
    }

    private func handleStartKnockLoop(result: @escaping FlutterResult) {
        #if canImport(NhpAgent)
        let count = IossdkNhpAgentKnockloopStart()
        result(count)
        #else
        result(FlutterError(code: "NOT_COMPILED", message: "NhpAgent framework not available", details: nil))
        #endif
    }

    private func handleStopKnockLoop(result: @escaping FlutterResult) {
        #if canImport(NhpAgent)
        IossdkNhpAgentKnockloopStop()
        result(nil)
        #else
        result(nil)
        #endif
    }

    private func handleGenerateKeys(args: [String: Any], result: @escaping FlutterResult) {
        #if canImport(NhpAgent)
        let cipherType = args["cipherType"] as? Int ?? 0
        let keys = IossdkNhpGenerateKeys(cipherType)
        result(keys)
        #else
        result(FlutterError(code: "NOT_COMPILED", message: "NhpAgent framework not available", details: nil))
        #endif
    }

    private func handlePrivateKeyToPublicKey(args: [String: Any], result: @escaping FlutterResult) {
        #if canImport(NhpAgent)
        let cipherType = args["cipherType"] as? Int ?? 0
        guard let privateKey = args["privateKey"] as? String else {
            result(FlutterError(code: "INVALID_ARGS", message: "privateKey is required", details: nil))
            return
        }
        let publicKey = IossdkNhpPrivkeyToPubkey(cipherType, privateKey)
        result(publicKey)
        #else
        result(FlutterError(code: "NOT_COMPILED", message: "NhpAgent framework not available", details: nil))
        #endif
    }
}
