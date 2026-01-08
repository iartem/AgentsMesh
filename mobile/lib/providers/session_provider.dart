import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:agentmesh/models/session.dart';
import 'package:agentmesh/providers/auth_provider.dart';

class SessionListState {
  final List<Session> sessions;
  final bool isLoading;
  final bool hasMore;
  final String? error;
  final String? statusFilter;

  const SessionListState({
    this.sessions = const [],
    this.isLoading = false,
    this.hasMore = true,
    this.error,
    this.statusFilter,
  });

  SessionListState copyWith({
    List<Session>? sessions,
    bool? isLoading,
    bool? hasMore,
    String? error,
    String? statusFilter,
  }) {
    return SessionListState(
      sessions: sessions ?? this.sessions,
      isLoading: isLoading ?? this.isLoading,
      hasMore: hasMore ?? this.hasMore,
      error: error,
      statusFilter: statusFilter ?? this.statusFilter,
    );
  }

  List<Session> get activeSessions =>
      sessions.where((s) => s.isActive).toList();

  List<Session> get completedSessions =>
      sessions.where((s) => s.isTerminated).toList();
}

class SessionListNotifier extends StateNotifier<SessionListState> {
  final Ref _ref;
  static const int _pageSize = 20;

  SessionListNotifier(this._ref) : super(const SessionListState());

  Future<void> loadSessions({bool refresh = false}) async {
    if (state.isLoading) return;

    if (refresh) {
      state = state.copyWith(sessions: [], hasMore: true);
    }

    state = state.copyWith(isLoading: true, error: null);

    try {
      final apiClient = _ref.read(apiClientProvider);
      final offset = refresh ? 0 : state.sessions.length;

      final response = await apiClient.getSessions(
        status: state.statusFilter,
        limit: _pageSize,
        offset: offset,
      );

      final List<dynamic> sessionList = response.data['sessions'] ?? [];
      final newSessions = sessionList.map((s) => Session.fromJson(s)).toList();

      state = state.copyWith(
        sessions: refresh ? newSessions : [...state.sessions, ...newSessions],
        isLoading: false,
        hasMore: newSessions.length >= _pageSize,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: 'Failed to load sessions',
      );
    }
  }

  Future<void> refresh() => loadSessions(refresh: true);

  void setStatusFilter(String? status) {
    state = state.copyWith(statusFilter: status);
    loadSessions(refresh: true);
  }
}

final sessionListProvider =
    StateNotifierProvider<SessionListNotifier, SessionListState>((ref) {
  return SessionListNotifier(ref);
});

class SessionDetailState {
  final Session? session;
  final bool isLoading;
  final String? error;

  const SessionDetailState({
    this.session,
    this.isLoading = false,
    this.error,
  });

  SessionDetailState copyWith({
    Session? session,
    bool? isLoading,
    String? error,
  }) {
    return SessionDetailState(
      session: session ?? this.session,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}

class SessionDetailNotifier extends StateNotifier<SessionDetailState> {
  final Ref _ref;
  final String sessionKey;

  SessionDetailNotifier(this._ref, this.sessionKey)
      : super(const SessionDetailState()) {
    loadSession();
  }

  Future<void> loadSession() async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final apiClient = _ref.read(apiClientProvider);
      final response = await apiClient.getSession(sessionKey);
      final session = Session.fromJson(response.data['session']);

      state = state.copyWith(
        session: session,
        isLoading: false,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: 'Failed to load session',
      );
    }
  }

  Future<bool> terminateSession() async {
    try {
      final apiClient = _ref.read(apiClientProvider);
      await apiClient.terminateSession(sessionKey);
      await loadSession();
      return true;
    } catch (e) {
      return false;
    }
  }

  void updateFromWebSocket(Session session) {
    if (session.sessionKey == sessionKey) {
      state = state.copyWith(session: session);
    }
  }
}

final sessionDetailProvider = StateNotifierProvider.family<
    SessionDetailNotifier, SessionDetailState, String>((ref, sessionKey) {
  return SessionDetailNotifier(ref, sessionKey);
});

// Runners provider
class RunnerListState {
  final List<Runner> runners;
  final bool isLoading;
  final String? error;

  const RunnerListState({
    this.runners = const [],
    this.isLoading = false,
    this.error,
  });

  RunnerListState copyWith({
    List<Runner>? runners,
    bool? isLoading,
    String? error,
  }) {
    return RunnerListState(
      runners: runners ?? this.runners,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }

  List<Runner> get onlineRunners => runners.where((r) => r.isOnline).toList();
  List<Runner> get availableRunners =>
      runners.where((r) => r.isOnline && r.hasCapacity).toList();
}

class RunnerListNotifier extends StateNotifier<RunnerListState> {
  final Ref _ref;

  RunnerListNotifier(this._ref) : super(const RunnerListState());

  Future<void> loadRunners() async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final apiClient = _ref.read(apiClientProvider);
      final response = await apiClient.getRunners();

      final List<dynamic> runnerList = response.data['runners'] ?? [];
      final runners = runnerList.map((r) => Runner.fromJson(r)).toList();

      state = state.copyWith(
        runners: runners,
        isLoading: false,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: 'Failed to load runners',
      );
    }
  }

  Future<void> refresh() => loadRunners();
}

final runnerListProvider =
    StateNotifierProvider<RunnerListNotifier, RunnerListState>((ref) {
  return RunnerListNotifier(ref);
});

// Agent types provider
class AgentTypeListState {
  final List<AgentType> types;
  final bool isLoading;
  final String? error;

  const AgentTypeListState({
    this.types = const [],
    this.isLoading = false,
    this.error,
  });

  AgentTypeListState copyWith({
    List<AgentType>? types,
    bool? isLoading,
    String? error,
  }) {
    return AgentTypeListState(
      types: types ?? this.types,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}

class AgentTypeListNotifier extends StateNotifier<AgentTypeListState> {
  final Ref _ref;

  AgentTypeListNotifier(this._ref) : super(const AgentTypeListState());

  Future<void> loadAgentTypes() async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final apiClient = _ref.read(apiClientProvider);
      final response = await apiClient.getAgentTypes();

      final List<dynamic> typeList = response.data['agent_types'] ?? [];
      final types = typeList.map((t) => AgentType.fromJson(t)).toList();

      state = state.copyWith(
        types: types,
        isLoading: false,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: 'Failed to load agent types',
      );
    }
  }
}

final agentTypeListProvider =
    StateNotifierProvider<AgentTypeListNotifier, AgentTypeListState>((ref) {
  return AgentTypeListNotifier(ref);
});
