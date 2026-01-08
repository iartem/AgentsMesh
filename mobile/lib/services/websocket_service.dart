import 'dart:async';
import 'dart:convert';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:agentmesh/utils/constants.dart';

enum WebSocketMessageType {
  ping,
  pong,
  sessionUpdate,
  terminalOutput,
  terminalInput,
  terminalResize,
  channelMessage,
  agentStatus,
  error,
}

class WebSocketMessage {
  final WebSocketMessageType type;
  final String? sessionId;
  final int? channelId;
  final dynamic data;
  final int timestamp;

  WebSocketMessage({
    required this.type,
    this.sessionId,
    this.channelId,
    this.data,
    required this.timestamp,
  });

  factory WebSocketMessage.fromJson(Map<String, dynamic> json) {
    return WebSocketMessage(
      type: _parseType(json['type'] as String),
      sessionId: json['session_id'] as String?,
      channelId: json['channel_id'] as int?,
      data: json['data'],
      timestamp: json['timestamp'] as int,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'type': _typeToString(type),
      if (sessionId != null) 'session_id': sessionId,
      if (channelId != null) 'channel_id': channelId,
      if (data != null) 'data': data,
      'timestamp': timestamp,
    };
  }

  static WebSocketMessageType _parseType(String type) {
    switch (type) {
      case 'ping':
        return WebSocketMessageType.ping;
      case 'pong':
        return WebSocketMessageType.pong;
      case 'session_update':
        return WebSocketMessageType.sessionUpdate;
      case 'terminal_output':
        return WebSocketMessageType.terminalOutput;
      case 'terminal_input':
        return WebSocketMessageType.terminalInput;
      case 'terminal_resize':
        return WebSocketMessageType.terminalResize;
      case 'channel_message':
        return WebSocketMessageType.channelMessage;
      case 'agent_status':
        return WebSocketMessageType.agentStatus;
      case 'error':
        return WebSocketMessageType.error;
      default:
        return WebSocketMessageType.error;
    }
  }

  static String _typeToString(WebSocketMessageType type) {
    switch (type) {
      case WebSocketMessageType.ping:
        return 'ping';
      case WebSocketMessageType.pong:
        return 'pong';
      case WebSocketMessageType.sessionUpdate:
        return 'session_update';
      case WebSocketMessageType.terminalOutput:
        return 'terminal_output';
      case WebSocketMessageType.terminalInput:
        return 'terminal_input';
      case WebSocketMessageType.terminalResize:
        return 'terminal_resize';
      case WebSocketMessageType.channelMessage:
        return 'channel_message';
      case WebSocketMessageType.agentStatus:
        return 'agent_status';
      case WebSocketMessageType.error:
        return 'error';
    }
  }
}

class WebSocketService {
  WebSocketChannel? _channel;
  StreamSubscription? _subscription;
  Timer? _pingTimer;
  Timer? _reconnectTimer;

  final _messageController = StreamController<WebSocketMessage>.broadcast();
  Stream<WebSocketMessage> get messages => _messageController.stream;

  final _connectionStateController = StreamController<bool>.broadcast();
  Stream<bool> get connectionState => _connectionStateController.stream;

  String? _token;
  String? _organizationSlug;
  bool _isConnected = false;
  int _reconnectAttempts = 0;
  static const int _maxReconnectAttempts = 5;

  bool get isConnected => _isConnected;

  Future<void> connect(String token, String organizationSlug) async {
    _token = token;
    _organizationSlug = organizationSlug;
    await _connect();
  }

  Future<void> _connect() async {
    if (_token == null || _organizationSlug == null) return;

    try {
      final wsUrl = ApiConstants.baseUrl
          .replaceFirst('https://', 'wss://')
          .replaceFirst('http://', 'ws://');

      final uri = Uri.parse(
          '$wsUrl/api/v1/ws/events?token=$_token&org=$_organizationSlug');

      _channel = WebSocketChannel.connect(uri);

      _subscription = _channel!.stream.listen(
        _onMessage,
        onError: _onError,
        onDone: _onDone,
      );

      _isConnected = true;
      _reconnectAttempts = 0;
      _connectionStateController.add(true);
      _startPingTimer();
    } catch (e) {
      _isConnected = false;
      _connectionStateController.add(false);
      _scheduleReconnect();
    }
  }

  void _onMessage(dynamic message) {
    try {
      final json = jsonDecode(message as String) as Map<String, dynamic>;
      final wsMessage = WebSocketMessage.fromJson(json);

      if (wsMessage.type == WebSocketMessageType.ping) {
        _sendPong();
        return;
      }

      _messageController.add(wsMessage);
    } catch (e) {
      // Invalid message format
    }
  }

  void _onError(dynamic error) {
    _isConnected = false;
    _connectionStateController.add(false);
    _scheduleReconnect();
  }

  void _onDone() {
    _isConnected = false;
    _connectionStateController.add(false);
    _scheduleReconnect();
  }

  void _startPingTimer() {
    _pingTimer?.cancel();
    _pingTimer = Timer.periodic(const Duration(seconds: 30), (_) {
      _sendPing();
    });
  }

  void _sendPing() {
    send(WebSocketMessage(
      type: WebSocketMessageType.ping,
      timestamp: DateTime.now().millisecondsSinceEpoch,
    ));
  }

  void _sendPong() {
    send(WebSocketMessage(
      type: WebSocketMessageType.pong,
      timestamp: DateTime.now().millisecondsSinceEpoch,
    ));
  }

  void send(WebSocketMessage message) {
    if (_isConnected && _channel != null) {
      _channel!.sink.add(jsonEncode(message.toJson()));
    }
  }

  void subscribeToSession(String sessionId) {
    send(WebSocketMessage(
      type: WebSocketMessageType.sessionUpdate,
      data: {'action': 'subscribe_session', 'session_id': sessionId},
      timestamp: DateTime.now().millisecondsSinceEpoch,
    ));
  }

  void unsubscribeFromSession(String sessionId) {
    send(WebSocketMessage(
      type: WebSocketMessageType.sessionUpdate,
      data: {'action': 'unsubscribe_session', 'session_id': sessionId},
      timestamp: DateTime.now().millisecondsSinceEpoch,
    ));
  }

  void subscribeToChannel(int channelId) {
    send(WebSocketMessage(
      type: WebSocketMessageType.channelMessage,
      data: {'action': 'subscribe_channel', 'channel_id': channelId},
      timestamp: DateTime.now().millisecondsSinceEpoch,
    ));
  }

  void unsubscribeFromChannel(int channelId) {
    send(WebSocketMessage(
      type: WebSocketMessageType.channelMessage,
      data: {'action': 'unsubscribe_channel', 'channel_id': channelId},
      timestamp: DateTime.now().millisecondsSinceEpoch,
    ));
  }

  void _scheduleReconnect() {
    if (_reconnectAttempts >= _maxReconnectAttempts) {
      return;
    }

    _reconnectTimer?.cancel();
    final delay = Duration(seconds: (1 << _reconnectAttempts).clamp(1, 32));
    _reconnectAttempts++;

    _reconnectTimer = Timer(delay, _connect);
  }

  Future<void> disconnect() async {
    _pingTimer?.cancel();
    _reconnectTimer?.cancel();
    await _subscription?.cancel();
    await _channel?.sink.close();
    _isConnected = false;
    _connectionStateController.add(false);
  }

  void dispose() {
    disconnect();
    _messageController.close();
    _connectionStateController.close();
  }
}
