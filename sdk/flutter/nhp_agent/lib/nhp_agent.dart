/// OpenNHP Agent SDK for Flutter
///
/// Provides Network-resource Hiding Protocol (NHP) functionality for
/// Zero Trust network access. This plugin wraps the native OpenNHP
/// agent SDKs for iOS and Android.
///
/// ## Quick Start
///
/// ```dart
/// // Initialize the agent
/// await NhpAgent.initialize(workingDir: '/path/to/config', logLevel: 2);
///
/// // Add server configuration
/// await NhpAgent.addServer(
///   pubkey: 'server-public-key-base64',
///   host: 'nhp.example.com',
///   port: 62206,
/// );
///
/// // Knock to access a protected resource
/// final result = await NhpAgent.knock(
///   authServiceId: 'discord',
///   resourceId: 'myapp',
///   serverHost: 'nhp.example.com',
///   serverPort: 62206,
/// );
///
/// if (result.success) {
///   print('Access granted for ${result.openTimeSeconds} seconds');
/// }
///
/// // Close when done
/// await NhpAgent.close();
/// ```
library nhp_agent;

import 'dart:async';
import 'dart:convert';
import 'package:flutter/services.dart';

/// Main class for interacting with the OpenNHP Agent.
///
/// All methods are static. Call [initialize] before using other methods.
class NhpAgent {
  static const MethodChannel _channel = MethodChannel('io.opennhp/nhp_agent');

  /// Whether the agent has been initialized.
  static bool _initialized = false;

  /// Returns true if the agent has been initialized.
  static bool get isInitialized => _initialized;

  /// Initialize the NHP agent with a working directory and log level.
  ///
  /// The working directory should contain:
  /// - `etc/` - Configuration files
  /// - `logs/` - Log files (created automatically)
  ///
  /// [workingDir] is the path to the agent's working directory.
  /// [logLevel] controls verbosity: 0=silent, 1=error, 2=info, 3=debug, 4=verbose
  ///
  /// Returns true if initialization succeeded.
  ///
  /// Throws [NhpException] if initialization fails.
  static Future<bool> initialize({
    required String workingDir,
    int logLevel = 2,
  }) async {
    try {
      final result = await _channel.invokeMethod<bool>('initialize', {
        'workingDir': workingDir,
        'logLevel': logLevel,
      });
      _initialized = result ?? false;
      return _initialized;
    } on PlatformException catch (e) {
      throw NhpException('Failed to initialize NHP agent: ${e.message}');
    }
  }

  /// Close the NHP agent and release resources.
  ///
  /// Call this when you're done using the agent.
  static Future<void> close() async {
    try {
      await _channel.invokeMethod('close');
      _initialized = false;
    } on PlatformException catch (e) {
      throw NhpException('Failed to close NHP agent: ${e.message}');
    }
  }

  /// Set the user information for knock requests.
  ///
  /// [userId] - User identification (recommended)
  /// [deviceId] - Device identification (optional)
  /// [orgId] - Organization identification (optional)
  /// [userData] - Additional JSON data for backend services (optional)
  ///
  /// Returns true if user info was set successfully.
  static Future<bool> setKnockUser({
    String? userId,
    String? deviceId,
    String? orgId,
    Map<String, dynamic>? userData,
  }) async {
    _checkInitialized();
    try {
      final result = await _channel.invokeMethod<bool>('setKnockUser', {
        'userId': userId ?? '',
        'deviceId': deviceId ?? '',
        'orgId': orgId ?? '',
        'userData': userData != null ? jsonEncode(userData) : '',
      });
      return result ?? false;
    } on PlatformException catch (e) {
      throw NhpException('Failed to set knock user: ${e.message}');
    }
  }

  /// Add an NHP server to the agent's server list.
  ///
  /// [pubkey] - Server's public key in base64 format (required)
  /// [ip] - Server IP address (optional if host is provided)
  /// [host] - Server hostname (optional if ip is provided)
  /// [port] - Server port (default: 62206)
  /// [expireTime] - Key expiration time in epoch seconds (0 = permanent)
  ///
  /// Returns true if server was added successfully.
  static Future<bool> addServer({
    required String pubkey,
    String? ip,
    String? host,
    int port = 62206,
    int expireTime = 0,
  }) async {
    _checkInitialized();
    if (ip == null && host == null) {
      throw NhpException('Either ip or host must be provided');
    }
    try {
      final result = await _channel.invokeMethod<bool>('addServer', {
        'pubkey': pubkey,
        'ip': ip ?? '',
        'host': host ?? '',
        'port': port,
        'expireTime': expireTime,
      });
      return result ?? false;
    } on PlatformException catch (e) {
      throw NhpException('Failed to add server: ${e.message}');
    }
  }

  /// Remove an NHP server from the agent's server list.
  ///
  /// [pubkey] - Server's public key to remove
  static Future<void> removeServer(String pubkey) async {
    _checkInitialized();
    try {
      await _channel.invokeMethod('removeServer', {'pubkey': pubkey});
    } on PlatformException catch (e) {
      throw NhpException('Failed to remove server: ${e.message}');
    }
  }

  /// Add a resource to the agent's resource list.
  ///
  /// [authServiceId] - Authentication service provider ID (e.g., 'discord')
  /// [resourceId] - Resource identifier
  /// [serverIp] - NHP server IP (optional if serverHost is provided)
  /// [serverHost] - NHP server hostname (optional if serverIp is provided)
  /// [serverPort] - NHP server port (default: 62206)
  ///
  /// Returns true if resource was added successfully.
  static Future<bool> addResource({
    required String authServiceId,
    required String resourceId,
    String? serverIp,
    String? serverHost,
    int serverPort = 62206,
  }) async {
    _checkInitialized();
    if (serverIp == null && serverHost == null) {
      throw NhpException('Either serverIp or serverHost must be provided');
    }
    try {
      final result = await _channel.invokeMethod<bool>('addResource', {
        'authServiceId': authServiceId,
        'resourceId': resourceId,
        'serverIp': serverIp ?? '',
        'serverHost': serverHost ?? '',
        'serverPort': serverPort,
      });
      return result ?? false;
    } on PlatformException catch (e) {
      throw NhpException('Failed to add resource: ${e.message}');
    }
  }

  /// Remove a resource from the agent's resource list.
  ///
  /// [authServiceId] - Authentication service provider ID
  /// [resourceId] - Resource identifier
  static Future<void> removeResource({
    required String authServiceId,
    required String resourceId,
  }) async {
    _checkInitialized();
    try {
      await _channel.invokeMethod('removeResource', {
        'authServiceId': authServiceId,
        'resourceId': resourceId,
      });
    } on PlatformException catch (e) {
      throw NhpException('Failed to remove resource: ${e.message}');
    }
  }

  /// Knock on a protected resource to request access.
  ///
  /// This is the main method for requesting access to NHP-protected resources.
  /// Before calling this, you must:
  /// 1. Initialize the agent with [initialize]
  /// 2. Add the server with [addServer]
  ///
  /// [authServiceId] - Authentication service provider ID (e.g., 'discord', 'subscription')
  /// [resourceId] - Resource identifier (e.g., 'downloads', 'cottonwood')
  /// [serverIp] - NHP server IP (optional if serverHost is provided)
  /// [serverHost] - NHP server hostname (optional if serverIp is provided)
  /// [serverPort] - NHP server port (default: 62206)
  ///
  /// Returns [NhpKnockResult] with the server's response.
  static Future<NhpKnockResult> knock({
    required String authServiceId,
    required String resourceId,
    String? serverIp,
    String? serverHost,
    int serverPort = 62206,
  }) async {
    _checkInitialized();
    if (serverIp == null && serverHost == null) {
      throw NhpException('Either serverIp or serverHost must be provided');
    }
    try {
      final result = await _channel.invokeMethod<String>('knock', {
        'authServiceId': authServiceId,
        'resourceId': resourceId,
        'serverIp': serverIp ?? '',
        'serverHost': serverHost ?? '',
        'serverPort': serverPort,
      });
      return NhpKnockResult.fromJson(result ?? '{}');
    } on PlatformException catch (e) {
      throw NhpException('Failed to knock: ${e.message}');
    }
  }

  /// Exit access to a protected resource.
  ///
  /// Explicitly informs the NHP server to revoke access.
  ///
  /// [authServiceId] - Authentication service provider ID
  /// [resourceId] - Resource identifier
  /// [serverIp] - NHP server IP (optional if serverHost is provided)
  /// [serverHost] - NHP server hostname (optional if serverIp is provided)
  /// [serverPort] - NHP server port (default: 62206)
  ///
  /// Returns true if exit was successful.
  static Future<bool> exitResource({
    required String authServiceId,
    required String resourceId,
    String? serverIp,
    String? serverHost,
    int serverPort = 62206,
  }) async {
    _checkInitialized();
    if (serverIp == null && serverHost == null) {
      throw NhpException('Either serverIp or serverHost must be provided');
    }
    try {
      final result = await _channel.invokeMethod<bool>('exitResource', {
        'authServiceId': authServiceId,
        'resourceId': resourceId,
        'serverIp': serverIp ?? '',
        'serverHost': serverHost ?? '',
        'serverPort': serverPort,
      });
      return result ?? false;
    } on PlatformException catch (e) {
      throw NhpException('Failed to exit resource: ${e.message}');
    }
  }

  /// Start the knock loop for continuous access.
  ///
  /// The knock loop automatically re-knocks resources before access expires.
  ///
  /// Returns the number of resources being knocked.
  static Future<int> startKnockLoop() async {
    _checkInitialized();
    try {
      final result = await _channel.invokeMethod<int>('startKnockLoop');
      return result ?? 0;
    } on PlatformException catch (e) {
      throw NhpException('Failed to start knock loop: ${e.message}');
    }
  }

  /// Stop the knock loop.
  static Future<void> stopKnockLoop() async {
    _checkInitialized();
    try {
      await _channel.invokeMethod('stopKnockLoop');
    } on PlatformException catch (e) {
      throw NhpException('Failed to stop knock loop: ${e.message}');
    }
  }

  /// Generate a new NHP key pair.
  ///
  /// [cipherType] - 0 for Curve25519, 1 for SM2
  ///
  /// Returns [NhpKeyPair] with private and public keys in base64.
  static Future<NhpKeyPair> generateKeys({int cipherType = 0}) async {
    try {
      final result = await _channel.invokeMethod<String>('generateKeys', {
        'cipherType': cipherType,
      });
      if (result == null || !result.contains('|')) {
        throw NhpException('Invalid key generation result');
      }
      final parts = result.split('|');
      return NhpKeyPair(
        privateKey: parts[0],
        publicKey: parts[1],
      );
    } on PlatformException catch (e) {
      throw NhpException('Failed to generate keys: ${e.message}');
    }
  }

  /// Derive public key from private key.
  ///
  /// [cipherType] - 0 for Curve25519, 1 for SM2
  /// [privateKey] - Private key in base64 format
  ///
  /// Returns the public key in base64 format.
  static Future<String> privateKeyToPublicKey({
    int cipherType = 0,
    required String privateKey,
  }) async {
    try {
      final result = await _channel.invokeMethod<String>('privateKeyToPublicKey', {
        'cipherType': cipherType,
        'privateKey': privateKey,
      });
      return result ?? '';
    } on PlatformException catch (e) {
      throw NhpException('Failed to derive public key: ${e.message}');
    }
  }

  static void _checkInitialized() {
    if (!_initialized) {
      throw NhpException('NHP agent not initialized. Call NhpAgent.initialize() first.');
    }
  }
}

/// Result of a knock request.
class NhpKnockResult {
  /// Whether the knock was successful (errCode == "0").
  final bool success;

  /// Error code from the server ("0" indicates success).
  final String errorCode;

  /// Error message from the server.
  final String? errorMessage;

  /// Resource host addresses.
  final Map<String, String>? resourceHosts;

  /// Duration of access in seconds.
  final int? openTimeSeconds;

  /// Token from the authentication service provider.
  final String? aspToken;

  /// Agent's IP address as seen by the server.
  final String? agentAddress;

  /// HTTP redirect URL (if any).
  final String? redirectUrl;

  NhpKnockResult({
    required this.success,
    required this.errorCode,
    this.errorMessage,
    this.resourceHosts,
    this.openTimeSeconds,
    this.aspToken,
    this.agentAddress,
    this.redirectUrl,
  });

  factory NhpKnockResult.fromJson(String jsonStr) {
    try {
      final map = jsonDecode(jsonStr) as Map<String, dynamic>;
      final errCode = map['errCode']?.toString() ?? '';
      return NhpKnockResult(
        success: errCode == '0' || errCode.isEmpty,
        errorCode: errCode,
        errorMessage: map['errMsg'] as String?,
        resourceHosts: map['resHost'] != null
            ? Map<String, String>.from(map['resHost'] as Map)
            : null,
        openTimeSeconds: map['opnTime'] as int?,
        aspToken: map['aspToken'] as String?,
        agentAddress: map['agentAddr'] as String?,
        redirectUrl: map['redirectUrl'] as String?,
      );
    } catch (e) {
      return NhpKnockResult(
        success: false,
        errorCode: 'PARSE_ERROR',
        errorMessage: 'Failed to parse knock result: $e',
      );
    }
  }

  @override
  String toString() {
    return 'NhpKnockResult(success: $success, errorCode: $errorCode, '
        'errorMessage: $errorMessage, openTimeSeconds: $openTimeSeconds)';
  }
}

/// NHP key pair for agent authentication.
class NhpKeyPair {
  /// Private key in base64 format.
  final String privateKey;

  /// Public key in base64 format.
  final String publicKey;

  NhpKeyPair({
    required this.privateKey,
    required this.publicKey,
  });
}

/// Exception thrown by NHP agent operations.
class NhpException implements Exception {
  final String message;

  NhpException(this.message);

  @override
  String toString() => 'NhpException: $message';
}
