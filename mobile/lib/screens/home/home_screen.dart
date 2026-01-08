import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:agentmesh/providers/auth_provider.dart';
import 'package:agentmesh/providers/organization_provider.dart';
import 'package:agentmesh/providers/ticket_provider.dart';
import 'package:agentmesh/providers/session_provider.dart';
import 'package:agentmesh/utils/theme.dart';
import 'package:agentmesh/widgets/organization_selector.dart';

class HomeScreen extends ConsumerStatefulWidget {
  const HomeScreen({super.key});

  @override
  ConsumerState<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends ConsumerState<HomeScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _loadData();
    });
  }

  Future<void> _loadData() async {
    await ref.read(organizationProvider.notifier).loadOrganizations();
    await Future.wait([
      ref.read(ticketListProvider.notifier).loadTickets(),
      ref.read(sessionListProvider.notifier).loadSessions(),
      ref.read(runnerListProvider.notifier).loadRunners(),
    ]);
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authProvider);
    final orgState = ref.watch(organizationProvider);
    final ticketState = ref.watch(ticketListProvider);
    final sessionState = ref.watch(sessionListProvider);
    final runnerState = ref.watch(runnerListProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('AgentMesh'),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: _loadData,
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: _loadData,
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            // Organization selector
            const OrganizationSelector(),
            const SizedBox(height: 24),

            // Welcome message
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        CircleAvatar(
                          backgroundImage: authState.user?.avatarUrl != null
                              ? NetworkImage(authState.user!.avatarUrl!)
                              : null,
                          child: authState.user?.avatarUrl == null
                              ? Text(
                                  authState.user?.displayName[0].toUpperCase() ??
                                      'U')
                              : null,
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                'Welcome back,',
                                style: Theme.of(context).textTheme.bodySmall,
                              ),
                              Text(
                                authState.user?.displayName ?? 'User',
                                style: Theme.of(context)
                                    .textTheme
                                    .titleMedium
                                    ?.copyWith(
                                      fontWeight: FontWeight.bold,
                                    ),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 16),

            // Quick stats
            Text(
              'Overview',
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: 12),

            // Stats cards
            Row(
              children: [
                Expanded(
                  child: _StatCard(
                    title: 'Active Sessions',
                    value: sessionState.activeSessions.length.toString(),
                    icon: Icons.terminal,
                    color: AppTheme.primaryColor,
                    onTap: () => context.go('/sessions'),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: _StatCard(
                    title: 'Open Tickets',
                    value: ticketState.tickets
                        .where((t) => !t.isCompleted)
                        .length
                        .toString(),
                    icon: Icons.task_alt,
                    color: AppTheme.warningColor,
                    onTap: () => context.go('/tickets'),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: _StatCard(
                    title: 'Online Runners',
                    value: runnerState.onlineRunners.length.toString(),
                    icon: Icons.computer,
                    color: AppTheme.successColor,
                    onTap: () {},
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: _StatCard(
                    title: 'Total Runners',
                    value: runnerState.runners.length.toString(),
                    icon: Icons.dns,
                    color: AppTheme.secondaryColor,
                    onTap: () {},
                  ),
                ),
              ],
            ),
            const SizedBox(height: 24),

            // Recent sessions
            _buildSectionHeader(
              context,
              'Active Sessions',
              onSeeAll: () => context.go('/sessions'),
            ),
            const SizedBox(height: 12),
            if (sessionState.isLoading && sessionState.sessions.isEmpty)
              const Center(child: CircularProgressIndicator())
            else if (sessionState.activeSessions.isEmpty)
              const _EmptyState(
                icon: Icons.terminal,
                message: 'No active sessions',
              )
            else
              ...sessionState.activeSessions.take(3).map((session) =>
                  _SessionCard(
                    session: session,
                    onTap: () => context.go('/sessions/${session.sessionKey}'),
                  )),

            const SizedBox(height: 24),

            // Recent tickets
            _buildSectionHeader(
              context,
              'Recent Tickets',
              onSeeAll: () => context.go('/tickets'),
            ),
            const SizedBox(height: 12),
            if (ticketState.isLoading && ticketState.tickets.isEmpty)
              const Center(child: CircularProgressIndicator())
            else if (ticketState.tickets.isEmpty)
              const _EmptyState(
                icon: Icons.task,
                message: 'No tickets',
              )
            else
              ...ticketState.tickets.take(5).map((ticket) => _TicketCard(
                    ticket: ticket,
                    onTap: () => context.go('/tickets/${ticket.identifier}'),
                  )),
          ],
        ),
      ),
    );
  }

  Widget _buildSectionHeader(
    BuildContext context,
    String title, {
    VoidCallback? onSeeAll,
  }) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          title,
          style: Theme.of(context).textTheme.titleMedium?.copyWith(
                fontWeight: FontWeight.bold,
              ),
        ),
        if (onSeeAll != null)
          TextButton(
            onPressed: onSeeAll,
            child: const Text('See All'),
          ),
      ],
    );
  }
}

class _StatCard extends StatelessWidget {
  final String title;
  final String value;
  final IconData icon;
  final Color color;
  final VoidCallback onTap;

  const _StatCard({
    required this.title,
    required this.value,
    required this.icon,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  color: color.withOpacity(0.1),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Icon(icon, color: color, size: 20),
              ),
              const SizedBox(height: 12),
              Text(
                value,
                style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
              ),
              const SizedBox(height: 4),
              Text(
                title,
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: Colors.grey,
                    ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _SessionCard extends StatelessWidget {
  final dynamic session;
  final VoidCallback onTap;

  const _SessionCard({
    required this.session,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        onTap: onTap,
        leading: Container(
          width: 40,
          height: 40,
          decoration: BoxDecoration(
            color: AppTheme.primaryColor.withOpacity(0.1),
            borderRadius: BorderRadius.circular(8),
          ),
          child: const Icon(Icons.terminal, color: AppTheme.primaryColor),
        ),
        title: Text(
          session.sessionKey,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
        ),
        subtitle: Text(
          'Status: ${session.status}',
          style: Theme.of(context).textTheme.bodySmall,
        ),
        trailing: _StatusChip(status: session.status),
      ),
    );
  }
}

class _TicketCard extends StatelessWidget {
  final dynamic ticket;
  final VoidCallback onTap;

  const _TicketCard({
    required this.ticket,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        onTap: onTap,
        leading: Container(
          width: 40,
          height: 40,
          decoration: BoxDecoration(
            color: _getPriorityColor(ticket.priority).withOpacity(0.1),
            borderRadius: BorderRadius.circular(8),
          ),
          child: Center(
            child: Text(
              ticket.identifier,
              style: Theme.of(context).textTheme.labelSmall?.copyWith(
                    fontWeight: FontWeight.bold,
                    color: _getPriorityColor(ticket.priority),
                  ),
            ),
          ),
        ),
        title: Text(
          ticket.title,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
        ),
        subtitle: Text(
          'Status: ${ticket.status}',
          style: Theme.of(context).textTheme.bodySmall,
        ),
        trailing: _StatusChip(status: ticket.status),
      ),
    );
  }

  Color _getPriorityColor(String priority) {
    switch (priority) {
      case 'urgent':
        return AppTheme.errorColor;
      case 'high':
        return Colors.orange;
      case 'medium':
        return AppTheme.warningColor;
      default:
        return Colors.grey;
    }
  }
}

class _StatusChip extends StatelessWidget {
  final String status;

  const _StatusChip({required this.status});

  @override
  Widget build(BuildContext context) {
    final color = _getStatusColor(status);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: color.withOpacity(0.1),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        status,
        style: Theme.of(context).textTheme.labelSmall?.copyWith(
              color: color,
              fontWeight: FontWeight.w500,
            ),
      ),
    );
  }

  Color _getStatusColor(String status) {
    switch (status) {
      case 'running':
      case 'in_progress':
        return AppTheme.primaryColor;
      case 'ready':
      case 'done':
        return AppTheme.successColor;
      case 'error':
      case 'cancelled':
        return AppTheme.errorColor;
      default:
        return Colors.grey;
    }
  }
}

class _EmptyState extends StatelessWidget {
  final IconData icon;
  final String message;

  const _EmptyState({
    required this.icon,
    required this.message,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          children: [
            Icon(icon, size: 48, color: Colors.grey),
            const SizedBox(height: 8),
            Text(
              message,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: Colors.grey,
                  ),
            ),
          ],
        ),
      ),
    );
  }
}
