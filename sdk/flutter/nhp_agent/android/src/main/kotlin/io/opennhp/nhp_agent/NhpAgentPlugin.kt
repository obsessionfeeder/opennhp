package io.opennhp.nhp_agent

import android.content.Context
import androidx.annotation.NonNull
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.MethodChannel.MethodCallHandler
import io.flutter.plugin.common.MethodChannel.Result
import kotlinx.coroutines.*

// Import the NhpAgent library (compiled from Go using gomobile)
// This will be available after running scripts/build_native.sh
// import nhpagent.Nhpagent

/**
 * OpenNHP Agent Flutter Plugin for Android
 *
 * Provides Network-resource Hiding Protocol (NHP) functionality
 * for Zero Trust network access.
 */
class NhpAgentPlugin: FlutterPlugin, MethodCallHandler {
    private lateinit var channel: MethodChannel
    private lateinit var context: Context
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    override fun onAttachedToEngine(@NonNull flutterPluginBinding: FlutterPlugin.FlutterPluginBinding) {
        channel = MethodChannel(flutterPluginBinding.binaryMessenger, "io.opennhp/nhp_agent")
        channel.setMethodCallHandler(this)
        context = flutterPluginBinding.applicationContext
    }

    override fun onMethodCall(@NonNull call: MethodCall, @NonNull result: Result) {
        when (call.method) {
            "initialize" -> handleInitialize(call, result)
            "close" -> handleClose(result)
            "setKnockUser" -> handleSetKnockUser(call, result)
            "addServer" -> handleAddServer(call, result)
            "removeServer" -> handleRemoveServer(call, result)
            "addResource" -> handleAddResource(call, result)
            "removeResource" -> handleRemoveResource(call, result)
            "knock" -> handleKnock(call, result)
            "exitResource" -> handleExitResource(call, result)
            "startKnockLoop" -> handleStartKnockLoop(result)
            "stopKnockLoop" -> handleStopKnockLoop(result)
            "generateKeys" -> handleGenerateKeys(call, result)
            "privateKeyToPublicKey" -> handlePrivateKeyToPublicKey(call, result)
            else -> result.notImplemented()
        }
    }

    override fun onDetachedFromEngine(@NonNull binding: FlutterPlugin.FlutterPluginBinding) {
        channel.setMethodCallHandler(null)
        scope.cancel()
    }

    // MARK: - Method Handlers

    private fun handleInitialize(call: MethodCall, result: Result) {
        val workingDir = call.argument<String>("workingDir")
        val logLevel = call.argument<Int>("logLevel") ?: 2

        if (workingDir == null) {
            result.error("INVALID_ARGS", "workingDir is required", null)
            return
        }

        try {
            // TODO: Uncomment when nhpagent library is compiled
            // val success = Nhpagent.nhpAgentInit(workingDir, logLevel.toLong())
            // result.success(success)

            // Placeholder until native library is compiled
            result.error("NOT_COMPILED", "NhpAgent library not available. Run scripts/build_native.sh first.", null)
        } catch (e: Exception) {
            result.error("INIT_ERROR", e.message, null)
        }
    }

    private fun handleClose(result: Result) {
        try {
            // TODO: Uncomment when nhpagent library is compiled
            // Nhpagent.nhpAgentClose()
            result.success(null)
        } catch (e: Exception) {
            result.error("CLOSE_ERROR", e.message, null)
        }
    }

    private fun handleSetKnockUser(call: MethodCall, result: Result) {
        val userId = call.argument<String>("userId") ?: ""
        val deviceId = call.argument<String>("deviceId") ?: ""
        val orgId = call.argument<String>("orgId") ?: ""
        val userData = call.argument<String>("userData") ?: ""

        try {
            // TODO: Uncomment when nhpagent library is compiled
            // val success = Nhpagent.nhpAgentSetKnockUser(userId, deviceId, orgId, userData)
            // result.success(success)

            result.error("NOT_COMPILED", "NhpAgent library not available", null)
        } catch (e: Exception) {
            result.error("SET_USER_ERROR", e.message, null)
        }
    }

    private fun handleAddServer(call: MethodCall, result: Result) {
        val pubkey = call.argument<String>("pubkey")
        val ip = call.argument<String>("ip") ?: ""
        val host = call.argument<String>("host") ?: ""
        val port = call.argument<Int>("port") ?: 62206
        val expireTime = call.argument<Long>("expireTime") ?: 0L

        if (pubkey == null) {
            result.error("INVALID_ARGS", "pubkey is required", null)
            return
        }

        try {
            // TODO: Uncomment when nhpagent library is compiled
            // val success = Nhpagent.nhpAgentAddServer(pubkey, ip, host, port.toLong(), expireTime)
            // result.success(success)

            result.error("NOT_COMPILED", "NhpAgent library not available", null)
        } catch (e: Exception) {
            result.error("ADD_SERVER_ERROR", e.message, null)
        }
    }

    private fun handleRemoveServer(call: MethodCall, result: Result) {
        val pubkey = call.argument<String>("pubkey") ?: ""

        try {
            // TODO: Uncomment when nhpagent library is compiled
            // Nhpagent.nhpAgentRemoveServer(pubkey)
            result.success(null)
        } catch (e: Exception) {
            result.error("REMOVE_SERVER_ERROR", e.message, null)
        }
    }

    private fun handleAddResource(call: MethodCall, result: Result) {
        val authServiceId = call.argument<String>("authServiceId")
        val resourceId = call.argument<String>("resourceId")
        val serverIp = call.argument<String>("serverIp") ?: ""
        val serverHost = call.argument<String>("serverHost") ?: ""
        val serverPort = call.argument<Int>("serverPort") ?: 62206

        if (authServiceId == null || resourceId == null) {
            result.error("INVALID_ARGS", "authServiceId and resourceId are required", null)
            return
        }

        try {
            // TODO: Uncomment when nhpagent library is compiled
            // val success = Nhpagent.nhpAgentAddResource(authServiceId, resourceId, serverIp, serverHost, serverPort.toLong())
            // result.success(success)

            result.error("NOT_COMPILED", "NhpAgent library not available", null)
        } catch (e: Exception) {
            result.error("ADD_RESOURCE_ERROR", e.message, null)
        }
    }

    private fun handleRemoveResource(call: MethodCall, result: Result) {
        val authServiceId = call.argument<String>("authServiceId") ?: ""
        val resourceId = call.argument<String>("resourceId") ?: ""

        try {
            // TODO: Uncomment when nhpagent library is compiled
            // Nhpagent.nhpAgentRemoveResource(authServiceId, resourceId)
            result.success(null)
        } catch (e: Exception) {
            result.error("REMOVE_RESOURCE_ERROR", e.message, null)
        }
    }

    private fun handleKnock(call: MethodCall, result: Result) {
        val authServiceId = call.argument<String>("authServiceId")
        val resourceId = call.argument<String>("resourceId")
        val serverIp = call.argument<String>("serverIp") ?: ""
        val serverHost = call.argument<String>("serverHost") ?: ""
        val serverPort = call.argument<Int>("serverPort") ?: 62206

        if (authServiceId == null || resourceId == null) {
            result.error("INVALID_ARGS", "authServiceId and resourceId are required", null)
            return
        }

        // Perform knock on background thread
        scope.launch {
            try {
                // TODO: Uncomment when nhpagent library is compiled
                // val jsonResult = Nhpagent.nhpAgentKnockResource(
                //     authServiceId, resourceId, serverIp, serverHost, serverPort.toLong()
                // )
                // withContext(Dispatchers.Main) {
                //     result.success(jsonResult)
                // }

                withContext(Dispatchers.Main) {
                    result.error("NOT_COMPILED", "NhpAgent library not available", null)
                }
            } catch (e: Exception) {
                withContext(Dispatchers.Main) {
                    result.error("KNOCK_ERROR", e.message, null)
                }
            }
        }
    }

    private fun handleExitResource(call: MethodCall, result: Result) {
        val authServiceId = call.argument<String>("authServiceId")
        val resourceId = call.argument<String>("resourceId")
        val serverIp = call.argument<String>("serverIp") ?: ""
        val serverHost = call.argument<String>("serverHost") ?: ""
        val serverPort = call.argument<Int>("serverPort") ?: 62206

        if (authServiceId == null || resourceId == null) {
            result.error("INVALID_ARGS", "authServiceId and resourceId are required", null)
            return
        }

        try {
            // TODO: Uncomment when nhpagent library is compiled
            // val success = Nhpagent.nhpAgentExitResource(
            //     authServiceId, resourceId, serverIp, serverHost, serverPort.toLong()
            // )
            // result.success(success)

            result.error("NOT_COMPILED", "NhpAgent library not available", null)
        } catch (e: Exception) {
            result.error("EXIT_RESOURCE_ERROR", e.message, null)
        }
    }

    private fun handleStartKnockLoop(result: Result) {
        try {
            // TODO: Uncomment when nhpagent library is compiled
            // val count = Nhpagent.nhpAgentKnockloopStart()
            // result.success(count.toInt())

            result.error("NOT_COMPILED", "NhpAgent library not available", null)
        } catch (e: Exception) {
            result.error("START_LOOP_ERROR", e.message, null)
        }
    }

    private fun handleStopKnockLoop(result: Result) {
        try {
            // TODO: Uncomment when nhpagent library is compiled
            // Nhpagent.nhpAgentKnockloopStop()
            result.success(null)
        } catch (e: Exception) {
            result.error("STOP_LOOP_ERROR", e.message, null)
        }
    }

    private fun handleGenerateKeys(call: MethodCall, result: Result) {
        val cipherType = call.argument<Int>("cipherType") ?: 0

        try {
            // TODO: Uncomment when nhpagent library is compiled
            // val keys = Nhpagent.nhpGenerateKeys(cipherType.toLong())
            // result.success(keys)

            result.error("NOT_COMPILED", "NhpAgent library not available", null)
        } catch (e: Exception) {
            result.error("GENERATE_KEYS_ERROR", e.message, null)
        }
    }

    private fun handlePrivateKeyToPublicKey(call: MethodCall, result: Result) {
        val cipherType = call.argument<Int>("cipherType") ?: 0
        val privateKey = call.argument<String>("privateKey")

        if (privateKey == null) {
            result.error("INVALID_ARGS", "privateKey is required", null)
            return
        }

        try {
            // TODO: Uncomment when nhpagent library is compiled
            // val publicKey = Nhpagent.nhpPrivkeyToPubkey(cipherType.toLong(), privateKey)
            // result.success(publicKey)

            result.error("NOT_COMPILED", "NhpAgent library not available", null)
        } catch (e: Exception) {
            result.error("DERIVE_KEY_ERROR", e.message, null)
        }
    }
}
