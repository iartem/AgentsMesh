import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:agentmesh/models/auth.dart';
import 'package:agentmesh/models/user.dart';
import 'package:agentmesh/services/api_client.dart';
import 'package:agentmesh/utils/constants.dart';

class AuthService {
  final ApiClient _apiClient;
  final FlutterSecureStorage _storage = const FlutterSecureStorage();

  AuthService(this._apiClient);

  Future<AuthResponse> login(String email, String password) async {
    final response = await _apiClient.login(email, password);
    final authResponse = AuthResponse.fromJson(response.data);

    await _saveTokens(authResponse);
    return authResponse;
  }

  Future<AuthResponse> register(
      String email, String username, String password, String? name) async {
    final response = await _apiClient.register(email, username, password, name);
    final authResponse = AuthResponse.fromJson(response.data);

    await _saveTokens(authResponse);
    return authResponse;
  }

  Future<void> logout() async {
    try {
      await _apiClient.logout();
    } catch (e) {
      // Ignore logout errors
    }
    await _clearTokens();
  }

  Future<User?> getCurrentUser() async {
    try {
      final response = await _apiClient.getCurrentUser();
      return User.fromJson(response.data['user']);
    } catch (e) {
      return null;
    }
  }

  Future<bool> isAuthenticated() async {
    final token = await _storage.read(key: StorageKeys.accessToken);
    return token != null;
  }

  Future<String?> getAccessToken() {
    return _storage.read(key: StorageKeys.accessToken);
  }

  Future<void> _saveTokens(AuthResponse response) async {
    await _storage.write(
        key: StorageKeys.accessToken, value: response.accessToken);
    if (response.refreshToken != null) {
      await _storage.write(
          key: StorageKeys.refreshToken, value: response.refreshToken);
    }
  }

  Future<void> _clearTokens() async {
    await _storage.delete(key: StorageKeys.accessToken);
    await _storage.delete(key: StorageKeys.refreshToken);
    await _storage.delete(key: StorageKeys.currentOrganization);
  }

  Future<OAuthConfig> getOAuthConfig(String provider) async {
    final response = await _apiClient.getOAuthConfig(provider);
    return OAuthConfig.fromJson(response.data);
  }

  Future<AuthResponse> handleOAuthCallback(String provider, String code) async {
    final response = await _apiClient.oauthCallback(provider, code);
    final authResponse = AuthResponse.fromJson(response.data);

    await _saveTokens(authResponse);
    return authResponse;
  }
}
