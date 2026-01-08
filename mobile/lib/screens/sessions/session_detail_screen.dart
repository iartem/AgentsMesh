import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:agentmesh/providers/session_provider.dart';
import 'package:agentmesh/utils/constants.dart';
import 'package:agentmesh/utils/theme.dart';
import 'package:intl/intl.dart';

class SessionDetailScreen extends ConsumerWidget {
  final String sessionKey;

  const SessionDetailScreen({super.key, required this.sessionKey});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sessionState = ref.watch(sessionDetailProvider(sessionKey));
    final session = sessionState.session;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Session Details'),
        actions: [
          if (session != null && session.isActive)
            IconButton(
              icon: const Icon(Icons.stop),
              onPressed: () => _showTerminateDialog(context, ref),
            ),
        ],
      ),
      body: sessionState.isLoading
          ? const Center(child: CircularProgressIndicator())
          : sessionState.error != null
              ? _buildError(context, ref, sessionState.error!)
              : session == null
                  ? const Center(child: Text('Session not found'))
                  : RefreshIndicator(
                      onRefresh: () => ref
                          .read(sessionDetailProvider(sessionKey).notifier)
                          .loadSession(),
                      child: SingleChildScrollView(
                        padding: const EdgeInsets.all(16),
                        physics: const AlwaysScrollableScrollPhysics(),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            // Status card
                            _buildStatusCard(context, session),
                            const SizedBox(height: 16),

                            // Session info
                            _buildInfoCard(context, session),
                            const SizedBox(height: 16),

                            // Runner info
                            if (session.runner != null)
                              _buildRunnerCard(context, session.runner!),
                            const SizedBox(height: 16),

                            // Agent info
                            _buildAgentCard(context, session),
                            const SizedBox(height: 16),

                            // Timing info
                            _buildTimingCard(context, session),

                            // Terminal preview (placeholder)
                            if (session.isActive) ...[
                              const SizedBox(height: 24),
                              _buildTerminalPreview(context),
                            ],
                          ],
                        ),
                      ),
                    ),
    );
  }

  Widget _buildError(BuildContext context, WidgetRef ref, String error) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.error_outline, size: 48, color: AppTheme.errorColor),
          const SizedBox(height: 16),
          Text(error),
          const SizedBox(height: 16),
          FilledButton(
            onPressed: () => ref
                .read(sessionDetailProvider(sessionKey).notifier)
                .loadSession(),
            child: const Text('Retry'),
          ),
        ],
      ),
    );
  }

  Widget _buildStatusCard(BuildContext context, dynamic session) {
    final color = _getStatusColor(session.status);
    final isActive = session.isActive;

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Row(
          children: [
            Container(
              width: 64,
              height: 64,
              decoration: BoxDecoration(
                color: color.withOpacity(0.1),
                borderRadius: BorderRadius.circular(16),
              ),
              child: Stack(
                children: [
                  Center(
                    child: Icon(Icons.terminal, color: color, size: 32),
                  ),
                  if (isActive)
                    Positioned(
                      right: 8,
                      top: 8,
                      child: Container(
                        width: 12,
                        height: 12,
                        decoration: BoxDecoration(
                          color: AppTheme.successColor,
                          shape: BoxShape.circle,
                          border: Border.all(
                            color: Theme.of(context).cardColor,
                            width: 2,
                          ),
                        ),
                      ),
                    ),
                ],
              ),
            ),
            const SizedBox(width: 16),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    SessionStatus.displayName(session.status),
                    style: Theme.of(context).textTheme.titleLarge?.copyWith(
                          fontWeight: FontWeight.bold,
                          color: color,
                        ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    session.sessionKey,
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: Colors.grey,
                          fontFamily: 'monospace',
                        ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildInfoCard(BuildContext context, dynamic session) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Session Information',
              style: Theme.of(context).textTheme.titleSmall?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: 16),
            _InfoRow(
              icon: Icons.key,
              label: 'Session Key',
              value: session.sessionKey,
              isMonospace: true,
            ),
            const Divider(),
            if (session.branchName != null) ...[
              _InfoRow(
                icon: Icons.fork_right,
                label: 'Branch',
                value: session.branchName!,
              ),
              const Divider(),
            ],
            if (session.initialPrompt != null) ...[
              _InfoRow(
                icon: Icons.message,
                label: 'Initial Prompt',
                value: session.initialPrompt!,
                maxLines: 3,
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildRunnerCard(BuildContext context, dynamic runner) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text(
                  'Runner',
                  style: Theme.of(context).textTheme.titleSmall?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                ),
                const Spacer(),
                Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                  decoration: BoxDecoration(
                    color: runner.isOnline
                        ? AppTheme.successColor.withOpacity(0.1)
                        : Colors.grey.withOpacity(0.1),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Text(
                    RunnerStatus.displayName(runner.status),
                    style: Theme.of(context).textTheme.labelSmall?.copyWith(
                          color: runner.isOnline
                              ? AppTheme.successColor
                              : Colors.grey,
                        ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
            _InfoRow(
              icon: Icons.computer,
              label: 'Node ID',
              value: runner.nodeId,
            ),
            if (runner.description != null) ...[
              const Divider(),
              _InfoRow(
                icon: Icons.description,
                label: 'Description',
                value: runner.description!,
              ),
            ],
            if (runner.os != null) ...[
              const Divider(),
              _InfoRow(
                icon: Icons.devices,
                label: 'Platform',
                value: '${runner.os} (${runner.arch})',
              ),
            ],
            if (runner.runnerVersion != null) ...[
              const Divider(),
              _InfoRow(
                icon: Icons.info,
                label: 'Version',
                value: runner.runnerVersion!,
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildAgentCard(BuildContext context, dynamic session) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Agent',
              style: Theme.of(context).textTheme.titleSmall?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: 16),
            _InfoRow(
              icon: Icons.smart_toy,
              label: 'Type',
              value: session.agentType?.name ?? 'Unknown',
            ),
            const Divider(),
            _InfoRow(
              icon: Icons.play_circle,
              label: 'Status',
              value: session.agentStatus,
              valueColor: _getAgentStatusColor(session.agentStatus),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildTimingCard(BuildContext context, dynamic session) {
    final dateFormat = DateFormat('MMM d, yyyy HH:mm:ss');

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Timing',
              style: Theme.of(context).textTheme.titleSmall?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: 16),
            _InfoRow(
              icon: Icons.schedule,
              label: 'Created',
              value: dateFormat.format(session.createdAt),
            ),
            if (session.startedAt != null) ...[
              const Divider(),
              _InfoRow(
                icon: Icons.play_arrow,
                label: 'Started',
                value: dateFormat.format(session.startedAt!),
              ),
            ],
            if (session.finishedAt != null) ...[
              const Divider(),
              _InfoRow(
                icon: Icons.stop,
                label: 'Finished',
                value: dateFormat.format(session.finishedAt!),
              ),
            ],
            if (session.duration != null) ...[
              const Divider(),
              _InfoRow(
                icon: Icons.timer,
                label: 'Duration',
                value: _formatDuration(session.duration!),
              ),
            ],
            if (session.lastActivity != null) ...[
              const Divider(),
              _InfoRow(
                icon: Icons.update,
                label: 'Last Activity',
                value: dateFormat.format(session.lastActivity!),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildTerminalPreview(BuildContext context) {
    return Card(
      color: const Color(0xFF1E1E1E),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Text(
                  'Terminal',
                  style: TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const Spacer(),
                OutlinedButton.icon(
                  onPressed: () {
                    // TODO: Open full terminal view
                  },
                  icon: const Icon(Icons.open_in_new, size: 16),
                  label: const Text('Open'),
                  style: OutlinedButton.styleFrom(
                    foregroundColor: Colors.white,
                    side: const BorderSide(color: Colors.white30),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
            Container(
              width: double.infinity,
              height: 150,
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: Colors.black,
                borderRadius: BorderRadius.circular(8),
              ),
              child: const Text(
                '> Session terminal output will appear here...\n\nUse the web interface for full terminal access.',
                style: TextStyle(
                  color: Color(0xFF00FF00),
                  fontFamily: 'monospace',
                  fontSize: 12,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _showTerminateDialog(BuildContext context, WidgetRef ref) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Terminate Session'),
        content: const Text(
          'Are you sure you want to terminate this session? This action cannot be undone.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () async {
              Navigator.pop(context);
              final success = await ref
                  .read(sessionDetailProvider(sessionKey).notifier)
                  .terminateSession();

              if (context.mounted) {
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(
                    content: Text(
                      success
                          ? 'Session terminated'
                          : 'Failed to terminate session',
                    ),
                  ),
                );
              }
            },
            style: FilledButton.styleFrom(
              backgroundColor: AppTheme.errorColor,
            ),
            child: const Text('Terminate'),
          ),
        ],
      ),
    );
  }

  Color _getStatusColor(String status) {
    switch (status) {
      case 'running':
        return AppTheme.successColor;
      case 'ready':
        return AppTheme.primaryColor;
      case 'initializing':
        return AppTheme.warningColor;
      case 'terminated':
        return Colors.grey;
      case 'error':
        return AppTheme.errorColor;
      default:
        return Colors.grey;
    }
  }

  Color _getAgentStatusColor(String status) {
    switch (status) {
      case 'running':
      case 'working':
        return AppTheme.successColor;
      case 'idle':
        return AppTheme.primaryColor;
      case 'waiting':
        return AppTheme.warningColor;
      case 'error':
        return AppTheme.errorColor;
      default:
        return Colors.grey;
    }
  }

  String _formatDuration(Duration duration) {
    if (duration.inHours > 0) {
      return '${duration.inHours}h ${duration.inMinutes % 60}m ${duration.inSeconds % 60}s';
    } else if (duration.inMinutes > 0) {
      return '${duration.inMinutes}m ${duration.inSeconds % 60}s';
    } else {
      return '${duration.inSeconds}s';
    }
  }
}

class _InfoRow extends StatelessWidget {
  final IconData icon;
  final String label;
  final String value;
  final bool isMonospace;
  final int maxLines;
  final Color? valueColor;

  const _InfoRow({
    required this.icon,
    required this.label,
    required this.value,
    this.isMonospace = false,
    this.maxLines = 1,
    this.valueColor,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 18, color: Colors.grey),
          const SizedBox(width: 12),
          SizedBox(
            width: 100,
            child: Text(
              label,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: Colors.grey,
                  ),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w500,
                    fontFamily: isMonospace ? 'monospace' : null,
                    color: valueColor,
                  ),
              maxLines: maxLines,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    );
  }
}
