import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:agentmesh/models/ticket.dart';
import 'package:agentmesh/providers/auth_provider.dart';

class TicketListState {
  final List<Ticket> tickets;
  final int total;
  final bool isLoading;
  final bool hasMore;
  final String? error;
  final TicketFilter filter;

  const TicketListState({
    this.tickets = const [],
    this.total = 0,
    this.isLoading = false,
    this.hasMore = true,
    this.error,
    this.filter = const TicketFilter(),
  });

  TicketListState copyWith({
    List<Ticket>? tickets,
    int? total,
    bool? isLoading,
    bool? hasMore,
    String? error,
    TicketFilter? filter,
  }) {
    return TicketListState(
      tickets: tickets ?? this.tickets,
      total: total ?? this.total,
      isLoading: isLoading ?? this.isLoading,
      hasMore: hasMore ?? this.hasMore,
      error: error,
      filter: filter ?? this.filter,
    );
  }
}

class TicketFilter {
  final int? teamId;
  final int? repositoryId;
  final String? status;
  final String? priority;
  final int? assigneeId;
  final String? query;

  const TicketFilter({
    this.teamId,
    this.repositoryId,
    this.status,
    this.priority,
    this.assigneeId,
    this.query,
  });

  TicketFilter copyWith({
    int? teamId,
    int? repositoryId,
    String? status,
    String? priority,
    int? assigneeId,
    String? query,
  }) {
    return TicketFilter(
      teamId: teamId ?? this.teamId,
      repositoryId: repositoryId ?? this.repositoryId,
      status: status ?? this.status,
      priority: priority ?? this.priority,
      assigneeId: assigneeId ?? this.assigneeId,
      query: query ?? this.query,
    );
  }

  TicketFilter clearFilter(String field) {
    switch (field) {
      case 'team':
        return TicketFilter(
          repositoryId: repositoryId,
          status: status,
          priority: priority,
          assigneeId: assigneeId,
          query: query,
        );
      case 'repository':
        return TicketFilter(
          teamId: teamId,
          status: status,
          priority: priority,
          assigneeId: assigneeId,
          query: query,
        );
      case 'status':
        return TicketFilter(
          teamId: teamId,
          repositoryId: repositoryId,
          priority: priority,
          assigneeId: assigneeId,
          query: query,
        );
      case 'priority':
        return TicketFilter(
          teamId: teamId,
          repositoryId: repositoryId,
          status: status,
          assigneeId: assigneeId,
          query: query,
        );
      case 'assignee':
        return TicketFilter(
          teamId: teamId,
          repositoryId: repositoryId,
          status: status,
          priority: priority,
          query: query,
        );
      default:
        return this;
    }
  }
}

class TicketListNotifier extends StateNotifier<TicketListState> {
  final Ref _ref;
  static const int _pageSize = 20;

  TicketListNotifier(this._ref) : super(const TicketListState());

  Future<void> loadTickets({bool refresh = false}) async {
    if (state.isLoading) return;

    if (refresh) {
      state = state.copyWith(tickets: [], hasMore: true);
    }

    state = state.copyWith(isLoading: true, error: null);

    try {
      final apiClient = _ref.read(apiClientProvider);
      final filter = state.filter;
      final offset = refresh ? 0 : state.tickets.length;

      final response = await apiClient.getTickets(
        teamId: filter.teamId,
        repositoryId: filter.repositoryId,
        status: filter.status,
        priority: filter.priority,
        assigneeId: filter.assigneeId,
        query: filter.query,
        limit: _pageSize,
        offset: offset,
      );

      final List<dynamic> ticketList = response.data['tickets'] ?? [];
      final newTickets = ticketList.map((t) => Ticket.fromJson(t)).toList();
      final total = response.data['total'] as int? ?? 0;

      state = state.copyWith(
        tickets: refresh ? newTickets : [...state.tickets, ...newTickets],
        total: total,
        isLoading: false,
        hasMore: newTickets.length >= _pageSize,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: 'Failed to load tickets',
      );
    }
  }

  Future<void> refresh() => loadTickets(refresh: true);

  void setFilter(TicketFilter filter) {
    state = state.copyWith(filter: filter);
    loadTickets(refresh: true);
  }

  void clearFilters() {
    state = state.copyWith(filter: const TicketFilter());
    loadTickets(refresh: true);
  }
}

final ticketListProvider =
    StateNotifierProvider<TicketListNotifier, TicketListState>((ref) {
  return TicketListNotifier(ref);
});

class TicketDetailState {
  final Ticket? ticket;
  final bool isLoading;
  final String? error;

  const TicketDetailState({
    this.ticket,
    this.isLoading = false,
    this.error,
  });

  TicketDetailState copyWith({
    Ticket? ticket,
    bool? isLoading,
    String? error,
  }) {
    return TicketDetailState(
      ticket: ticket ?? this.ticket,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}

class TicketDetailNotifier extends StateNotifier<TicketDetailState> {
  final Ref _ref;
  final String identifier;

  TicketDetailNotifier(this._ref, this.identifier)
      : super(const TicketDetailState()) {
    loadTicket();
  }

  Future<void> loadTicket() async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final apiClient = _ref.read(apiClientProvider);
      final response = await apiClient.getTicket(identifier);
      final ticket = Ticket.fromJson(response.data['ticket']);

      state = state.copyWith(
        ticket: ticket,
        isLoading: false,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: 'Failed to load ticket',
      );
    }
  }

  Future<bool> updateTicket(UpdateTicketRequest request) async {
    try {
      final apiClient = _ref.read(apiClientProvider);
      final response =
          await apiClient.updateTicket(identifier, request.toJson());
      final ticket = Ticket.fromJson(response.data['ticket']);

      state = state.copyWith(ticket: ticket);
      return true;
    } catch (e) {
      return false;
    }
  }

  Future<bool> updateStatus(String status) async {
    return updateTicket(UpdateTicketRequest(status: status));
  }
}

final ticketDetailProvider = StateNotifierProvider.family<TicketDetailNotifier,
    TicketDetailState, String>((ref, identifier) {
  return TicketDetailNotifier(ref, identifier);
});
