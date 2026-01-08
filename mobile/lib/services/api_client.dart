import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:agentmesh/utils/constants.dart';

class ApiClient {
  late final Dio _dio;
  final FlutterSecureStorage _storage = const FlutterSecureStorage();
  String? _currentOrganization;

  ApiClient() {
    _dio = Dio(BaseOptions(
      baseUrl: ApiConstants.baseUrl,
      connectTimeout: ApiConstants.connectTimeout,
      receiveTimeout: ApiConstants.receiveTimeout,
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
      },
    ));

    _dio.interceptors.add(InterceptorsWrapper(
      onRequest: _onRequest,
      onResponse: _onResponse,
      onError: _onError,
    ));
  }

  Future<void> _onRequest(
      RequestOptions options, RequestInterceptorHandler handler) async {
    // Add auth token
    final token = await _storage.read(key: StorageKeys.accessToken);
    if (token != null) {
      options.headers['Authorization'] = 'Bearer $token';
    }

    // Add organization context
    if (_currentOrganization != null) {
      options.headers['X-Organization-Slug'] = _currentOrganization;
    }

    handler.next(options);
  }

  void _onResponse(Response response, ResponseInterceptorHandler handler) {
    handler.next(response);
  }

  Future<void> _onError(
      DioException error, ErrorInterceptorHandler handler) async {
    if (error.response?.statusCode == 401) {
      // Try to refresh token
      final refreshed = await _refreshToken();
      if (refreshed) {
        // Retry request
        final options = error.requestOptions;
        final token = await _storage.read(key: StorageKeys.accessToken);
        options.headers['Authorization'] = 'Bearer $token';

        try {
          final response = await _dio.fetch(options);
          handler.resolve(response);
          return;
        } catch (e) {
          // Refresh failed, proceed with error
        }
      }
    }
    handler.next(error);
  }

  Future<bool> _refreshToken() async {
    try {
      final refreshToken = await _storage.read(key: StorageKeys.refreshToken);
      if (refreshToken == null) return false;

      final response = await _dio.post(
        '/api/v1/auth/refresh',
        data: {'refresh_token': refreshToken},
        options: Options(headers: {'Authorization': null}),
      );

      if (response.statusCode == 200) {
        final data = response.data;
        await _storage.write(
            key: StorageKeys.accessToken, value: data['access_token']);
        if (data['refresh_token'] != null) {
          await _storage.write(
              key: StorageKeys.refreshToken, value: data['refresh_token']);
        }
        return true;
      }
    } catch (e) {
      // Refresh failed
    }
    return false;
  }

  void setOrganization(String? slug) {
    _currentOrganization = slug;
  }

  // Auth endpoints
  Future<Response> login(String email, String password) {
    return _dio.post('/api/v1/auth/login', data: {
      'email': email,
      'password': password,
    });
  }

  Future<Response> register(
      String email, String username, String password, String? name) {
    return _dio.post('/api/v1/auth/register', data: {
      'email': email,
      'username': username,
      'password': password,
      if (name != null) 'name': name,
    });
  }

  Future<Response> logout() {
    return _dio.post('/api/v1/auth/logout');
  }

  Future<Response> getOAuthConfig(String provider) {
    return _dio.get('/api/v1/auth/oauth/$provider/config');
  }

  Future<Response> oauthCallback(String provider, String code) {
    return _dio.post('/api/v1/auth/oauth/$provider/callback', data: {
      'code': code,
    });
  }

  // User endpoints
  Future<Response> getCurrentUser() {
    return _dio.get('/api/v1/users/me');
  }

  Future<Response> updateProfile(Map<String, dynamic> data) {
    return _dio.put('/api/v1/users/me', data: data);
  }

  Future<Response> getUserOrganizations() {
    return _dio.get('/api/v1/users/me/organizations');
  }

  // Organization endpoints
  Future<Response> getOrganization(String slug) {
    return _dio.get('/api/v1/organizations/$slug');
  }

  Future<Response> getOrganizationMembers(String slug) {
    return _dio.get('/api/v1/organizations/$slug/members');
  }

  // Team endpoints
  Future<Response> getTeams({int? limit, int? offset}) {
    return _dio.get('/api/v1/teams', queryParameters: {
      if (limit != null) 'limit': limit,
      if (offset != null) 'offset': offset,
    });
  }

  Future<Response> getTeam(int id) {
    return _dio.get('/api/v1/teams/$id');
  }

  // Ticket endpoints
  Future<Response> getTickets({
    int? teamId,
    int? repositoryId,
    String? status,
    String? priority,
    int? assigneeId,
    String? query,
    int? limit,
    int? offset,
  }) {
    return _dio.get('/api/v1/tickets', queryParameters: {
      if (teamId != null) 'team_id': teamId,
      if (repositoryId != null) 'repository_id': repositoryId,
      if (status != null) 'status': status,
      if (priority != null) 'priority': priority,
      if (assigneeId != null) 'assignee_id': assigneeId,
      if (query != null) 'query': query,
      if (limit != null) 'limit': limit,
      if (offset != null) 'offset': offset,
    });
  }

  Future<Response> getTicket(String identifier) {
    return _dio.get('/api/v1/tickets/$identifier');
  }

  Future<Response> createTicket(Map<String, dynamic> data) {
    return _dio.post('/api/v1/tickets', data: data);
  }

  Future<Response> updateTicket(String identifier, Map<String, dynamic> data) {
    return _dio.put('/api/v1/tickets/$identifier', data: data);
  }

  Future<Response> deleteTicket(String identifier) {
    return _dio.delete('/api/v1/tickets/$identifier');
  }

  Future<Response> getLabels({int? repositoryId}) {
    return _dio.get('/api/v1/tickets/labels', queryParameters: {
      if (repositoryId != null) 'repository_id': repositoryId,
    });
  }

  // Session endpoints
  Future<Response> getSessions({
    int? teamId,
    String? status,
    int? limit,
    int? offset,
  }) {
    return _dio.get('/api/v1/sessions', queryParameters: {
      if (teamId != null) 'team_id': teamId,
      if (status != null) 'status': status,
      if (limit != null) 'limit': limit,
      if (offset != null) 'offset': offset,
    });
  }

  Future<Response> getSession(String key) {
    return _dio.get('/api/v1/sessions/$key');
  }

  Future<Response> createSession(Map<String, dynamic> data) {
    return _dio.post('/api/v1/sessions', data: data);
  }

  Future<Response> terminateSession(String key) {
    return _dio.post('/api/v1/sessions/$key/terminate');
  }

  // Runner endpoints
  Future<Response> getRunners({String? status, int? limit, int? offset}) {
    return _dio.get('/api/v1/runners', queryParameters: {
      if (status != null) 'status': status,
      if (limit != null) 'limit': limit,
      if (offset != null) 'offset': offset,
    });
  }

  Future<Response> getRunner(int id) {
    return _dio.get('/api/v1/runners/$id');
  }

  // Agent endpoints
  Future<Response> getAgentTypes() {
    return _dio.get('/api/v1/agents/types');
  }

  // Channel endpoints
  Future<Response> getChannels({
    int? teamId,
    bool? includeArchived,
    int? limit,
    int? offset,
  }) {
    return _dio.get('/api/v1/channels', queryParameters: {
      if (teamId != null) 'team_id': teamId,
      if (includeArchived != null) 'include_archived': includeArchived,
      if (limit != null) 'limit': limit,
      if (offset != null) 'offset': offset,
    });
  }

  Future<Response> getChannel(int id) {
    return _dio.get('/api/v1/channels/$id');
  }

  Future<Response> getChannelMessages(int id, {DateTime? before, int? limit}) {
    return _dio.get('/api/v1/channels/$id/messages', queryParameters: {
      if (before != null) 'before': before.toIso8601String(),
      if (limit != null) 'limit': limit,
    });
  }

  Future<Response> sendChannelMessage(int id, Map<String, dynamic> data) {
    return _dio.post('/api/v1/channels/$id/messages', data: data);
  }

  // Billing endpoints
  Future<Response> getBillingOverview() {
    return _dio.get('/api/v1/billing/overview');
  }

  Future<Response> getSubscription() {
    return _dio.get('/api/v1/billing/subscription');
  }

  Future<Response> getUsage({String? type}) {
    return _dio.get('/api/v1/billing/usage', queryParameters: {
      if (type != null) 'type': type,
    });
  }

  Future<Response> getPlans() {
    return _dio.get('/api/v1/billing/plans');
  }
}
