# OpenNHP Agent SDK for Flutter

A Flutter plugin that provides Network-resource Hiding Protocol (NHP) functionality for Zero Trust network access.

## Features

- **Zero Trust Authentication**: Knock protected resources to gain temporary access
- **IP Whitelisting**: Server adds your IP to whitelist after successful knock
- **True Invisibility**: Protected resources return no response to unauthorized users
- **Cross-Platform**: Works on iOS and Android

## Installation

### 1. Add Dependency

Add to your `pubspec.yaml`:

```yaml
dependencies:
  nhp_agent:
    path: ../opennhp/sdk/flutter/nhp_agent
```

### 2. Build Native SDKs

The plugin requires native Go libraries compiled with gomobile.

```bash
cd opennhp/sdk/flutter/nhp_agent
./scripts/build_native.sh
```

This will build:
- iOS: `ios/Frameworks/NhpAgent.xcframework`
- Android: `android/src/main/libs/nhpagent.aar`

### 3. iOS Setup

No additional setup required. The xcframework is automatically linked.

### 4. Android Setup

Add to your app's `android/app/build.gradle`:

```gradle
android {
    // ...
    packagingOptions {
        pickFirst 'lib/*/libnhpagent.so'
    }
}
```

## Usage

### Basic Usage

```dart
import 'package:nhp_agent/nhp_agent.dart';

// Initialize the agent
final configPath = await _setupConfig();
await NhpAgent.initialize(workingDir: configPath, logLevel: 2);

// Add the NHP server
await NhpAgent.addServer(
  pubkey: 'your-server-public-key-base64',
  host: 'nhp.example.com',
  port: 62206,
);

// Knock to access a protected resource
final result = await NhpAgent.knock(
  authServiceId: 'subscription',
  resourceId: 'downloads',
  serverHost: 'nhp.example.com',
  serverPort: 62206,
);

if (result.success) {
  print('Access granted for ${result.openTimeSeconds} seconds');
  // Now you can access the protected resource
  // Your IP has been whitelisted
} else {
  print('Knock failed: ${result.errorMessage}');
}

// Clean up when done
await NhpAgent.close();
```

### Setting User Information

```dart
await NhpAgent.setKnockUser(
  userId: 'user123',
  deviceId: 'device-uuid',
  orgId: 'myorg',
  userData: {'premium': true},
);
```

### Continuous Access with Knock Loop

```dart
// Add resources to monitor
await NhpAgent.addResource(
  authServiceId: 'subscription',
  resourceId: 'downloads',
  serverHost: 'nhp.example.com',
);

// Start automatic re-knocking
final count = await NhpAgent.startKnockLoop();
print('Monitoring $count resources');

// Stop when done
await NhpAgent.stopKnockLoop();
```

### Key Generation

```dart
// Generate a new key pair
final keyPair = await NhpAgent.generateKeys(cipherType: 0); // 0=Curve25519
print('Private: ${keyPair.privateKey}');
print('Public: ${keyPair.publicKey}');
```

## Configuration

The agent requires configuration files in the working directory:

```
workingDir/
├── etc/
│   ├── config.toml     # Agent configuration
│   └── resource.toml   # Resource definitions
└── logs/               # Created automatically
```

### Example config.toml

```toml
AgentId = "flutter-app"
DefaultCipherScheme = 0
LogLevel = 2
```

## API Reference

### NhpAgent

| Method | Description |
|--------|-------------|
| `initialize()` | Initialize the agent |
| `close()` | Release resources |
| `setKnockUser()` | Set user information |
| `addServer()` | Add NHP server |
| `removeServer()` | Remove NHP server |
| `addResource()` | Add resource to monitor |
| `removeResource()` | Remove resource |
| `knock()` | Request access to resource |
| `exitResource()` | Revoke access |
| `startKnockLoop()` | Start auto-knock |
| `stopKnockLoop()` | Stop auto-knock |
| `generateKeys()` | Generate key pair |

### NhpKnockResult

| Property | Type | Description |
|----------|------|-------------|
| `success` | bool | Whether knock succeeded |
| `errorCode` | String | Error code ("0" = success) |
| `errorMessage` | String? | Error description |
| `openTimeSeconds` | int? | Access duration |
| `redirectUrl` | String? | Redirect URL if any |

## Security

- Private keys are stored securely on device
- All communication uses NHP's cryptographic protocol
- IP whitelist expires automatically after `openTimeSeconds`

## License

Apache 2.0 - See [LICENSE](../../../LICENSE)
