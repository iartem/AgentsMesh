import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:agentmesh/providers/ticket_provider.dart';
import 'package:agentmesh/utils/constants.dart';
import 'package:agentmesh/utils/theme.dart';
import 'package:intl/intl.dart';
import 'package:url_launcher/url_launcher.dart';

class TicketDetailScreen extends ConsumerWidget {
  final String identifier;

  const TicketDetailScreen({super.key, required this.identifier});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ticketState = ref.watch(ticketDetailProvider(identifier));
    final ticket = ticketState.ticket;

    return Scaffold(
      appBar: AppBar(
        title: Text(identifier),
        actions: [
          IconButton(
            icon: const Icon(Icons.edit),
            onPressed: ticket == null
                ? null
                : () {
                    // TODO: Navigate to edit screen
                  },
          ),
          PopupMenuButton<String>(
            onSelected: (value) async {
              if (value == 'delete') {
                // TODO: Implement delete
              }
            },
            itemBuilder: (context) => [
              const PopupMenuItem(
                value: 'delete',
                child: Row(
                  children: [
                    Icon(Icons.delete, color: Colors.red),
                    SizedBox(width: 8),
                    Text('Delete', style: TextStyle(color: Colors.red)),
                  ],
                ),
              ),
            ],
          ),
        ],
      ),
      body: ticketState.isLoading
          ? const Center(child: CircularProgressIndicator())
          : ticketState.error != null
              ? _buildError(context, ref, ticketState.error!)
              : ticket == null
                  ? const Center(child: Text('Ticket not found'))
                  : RefreshIndicator(
                      onRefresh: () => ref
                          .read(ticketDetailProvider(identifier).notifier)
                          .loadTicket(),
                      child: SingleChildScrollView(
                        padding: const EdgeInsets.all(16),
                        physics: const AlwaysScrollableScrollPhysics(),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            // Title and status
                            _buildHeader(context, ticket),
                            const SizedBox(height: 24),

                            // Quick actions
                            _buildQuickActions(context, ref, ticket),
                            const SizedBox(height: 24),

                            // Details
                            _buildDetails(context, ticket),
                            const SizedBox(height: 24),

                            // Description
                            if (ticket.description != null) ...[
                              _buildSection(
                                context,
                                'Description',
                                child: Text(ticket.description!),
                              ),
                              const SizedBox(height: 24),
                            ],

                            // Labels
                            if (ticket.labels != null &&
                                ticket.labels!.isNotEmpty) ...[
                              _buildSection(
                                context,
                                'Labels',
                                child: Wrap(
                                  spacing: 8,
                                  runSpacing: 8,
                                  children: ticket.labels!
                                      .map((label) => _LabelChip(label: label))
                                      .toList(),
                                ),
                              ),
                              const SizedBox(height: 24),
                            ],

                            // Assignees
                            if (ticket.assignees != null &&
                                ticket.assignees!.isNotEmpty) ...[
                              _buildSection(
                                context,
                                'Assignees',
                                child: Wrap(
                                  spacing: 8,
                                  runSpacing: 8,
                                  children: ticket.assignees!
                                      .map((a) =>
                                          _AssigneeChip(assignee: a))
                                      .toList(),
                                ),
                              ),
                              const SizedBox(height: 24),
                            ],

                            // Merge Requests
                            if (ticket.mergeRequests != null &&
                                ticket.mergeRequests!.isNotEmpty) ...[
                              _buildSection(
                                context,
                                'Merge Requests',
                                child: Column(
                                  children: ticket.mergeRequests!
                                      .map((mr) => _MergeRequestItem(mr: mr))
                                      .toList(),
                                ),
                              ),
                              const SizedBox(height: 24),
                            ],

                            // Sub-tickets
                            if (ticket.subTickets != null &&
                                ticket.subTickets!.isNotEmpty) ...[
                              _buildSection(
                                context,
                                'Sub-tickets',
                                child: Column(
                                  children: ticket.subTickets!
                                      .map((t) => _SubTicketItem(ticket: t))
                                      .toList(),
                                ),
                              ),
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
                .read(ticketDetailProvider(identifier).notifier)
                .loadTicket(),
            child: const Text('Retry'),
          ),
        ],
      ),
    );
  }

  Widget _buildHeader(BuildContext context, dynamic ticket) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            _StatusBadge(status: ticket.status),
            const SizedBox(width: 8),
            _PriorityBadge(priority: ticket.priority),
            const Spacer(),
            Text(
              '#${ticket.number}',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: Colors.grey,
                  ),
            ),
          ],
        ),
        const SizedBox(height: 12),
        Text(
          ticket.title,
          style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                fontWeight: FontWeight.bold,
              ),
        ),
      ],
    );
  }

  Widget _buildQuickActions(
      BuildContext context, WidgetRef ref, dynamic ticket) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              'Change Status',
              style: Theme.of(context).textTheme.titleSmall,
            ),
            const SizedBox(height: 12),
            SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                children: [
                  for (final status in [
                    'backlog',
                    'todo',
                    'in_progress',
                    'in_review',
                    'done'
                  ])
                    Padding(
                      padding: const EdgeInsets.only(right: 8),
                      child: ChoiceChip(
                        label: Text(TicketStatus.displayName(status)),
                        selected: ticket.status == status,
                        onSelected: ticket.status == status
                            ? null
                            : (selected) {
                                if (selected) {
                                  ref
                                      .read(ticketDetailProvider(identifier)
                                          .notifier)
                                      .updateStatus(status);
                                }
                              },
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildDetails(BuildContext context, dynamic ticket) {
    final dateFormat = DateFormat('MMM d, yyyy');

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            _DetailRow(
              icon: Icons.category,
              label: 'Type',
              value: ticket.type,
            ),
            const Divider(),
            _DetailRow(
              icon: Icons.person,
              label: 'Reporter',
              value: ticket.reporter?.displayName ?? 'Unknown',
            ),
            const Divider(),
            _DetailRow(
              icon: Icons.calendar_today,
              label: 'Created',
              value: dateFormat.format(ticket.createdAt),
            ),
            if (ticket.dueDate != null) ...[
              const Divider(),
              _DetailRow(
                icon: Icons.event,
                label: 'Due Date',
                value: dateFormat.format(ticket.dueDate!),
                valueColor: ticket.isOverdue ? AppTheme.errorColor : null,
              ),
            ],
            if (ticket.startedAt != null) ...[
              const Divider(),
              _DetailRow(
                icon: Icons.play_arrow,
                label: 'Started',
                value: dateFormat.format(ticket.startedAt!),
              ),
            ],
            if (ticket.completedAt != null) ...[
              const Divider(),
              _DetailRow(
                icon: Icons.check_circle,
                label: 'Completed',
                value: dateFormat.format(ticket.completedAt!),
                valueColor: AppTheme.successColor,
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildSection(BuildContext context, String title, {required Widget child}) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: Theme.of(context).textTheme.titleMedium?.copyWith(
                fontWeight: FontWeight.bold,
              ),
        ),
        const SizedBox(height: 12),
        child,
      ],
    );
  }
}

class _StatusBadge extends StatelessWidget {
  final String status;

  const _StatusBadge({required this.status});

  @override
  Widget build(BuildContext context) {
    final color = _getStatusColor(status);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      decoration: BoxDecoration(
        color: color.withOpacity(0.1),
        borderRadius: BorderRadius.circular(16),
      ),
      child: Text(
        TicketStatus.displayName(status),
        style: Theme.of(context).textTheme.labelMedium?.copyWith(
              color: color,
              fontWeight: FontWeight.w600,
            ),
      ),
    );
  }

  Color _getStatusColor(String status) {
    switch (status) {
      case 'in_progress':
      case 'in_review':
        return AppTheme.primaryColor;
      case 'done':
        return AppTheme.successColor;
      case 'cancelled':
        return AppTheme.errorColor;
      case 'todo':
        return AppTheme.warningColor;
      default:
        return Colors.grey;
    }
  }
}

class _PriorityBadge extends StatelessWidget {
  final String priority;

  const _PriorityBadge({required this.priority});

  @override
  Widget build(BuildContext context) {
    final color = _getPriorityColor(priority);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        border: Border.all(color: color),
        borderRadius: BorderRadius.circular(16),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.flag, size: 14, color: color),
          const SizedBox(width: 4),
          Text(
            TicketPriority.displayName(priority),
            style: Theme.of(context).textTheme.labelSmall?.copyWith(
                  color: color,
                ),
          ),
        ],
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
      case 'low':
        return Colors.blue;
      default:
        return Colors.grey;
    }
  }
}

class _DetailRow extends StatelessWidget {
  final IconData icon;
  final String label;
  final String value;
  final Color? valueColor;

  const _DetailRow({
    required this.icon,
    required this.label,
    required this.value,
    this.valueColor,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Row(
        children: [
          Icon(icon, size: 20, color: Colors.grey),
          const SizedBox(width: 12),
          Text(
            label,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Colors.grey,
                ),
          ),
          const Spacer(),
          Text(
            value,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w500,
                  color: valueColor,
                ),
          ),
        ],
      ),
    );
  }
}

class _LabelChip extends StatelessWidget {
  final dynamic label;

  const _LabelChip({required this.label});

  @override
  Widget build(BuildContext context) {
    final color = _parseColor(label.color);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      decoration: BoxDecoration(
        color: color.withOpacity(0.2),
        borderRadius: BorderRadius.circular(16),
      ),
      child: Text(
        label.name,
        style: Theme.of(context).textTheme.labelMedium?.copyWith(
              color: color,
              fontWeight: FontWeight.w500,
            ),
      ),
    );
  }

  Color _parseColor(String hex) {
    try {
      return Color(int.parse(hex.replaceFirst('#', '0xFF')));
    } catch (e) {
      return Colors.grey;
    }
  }
}

class _AssigneeChip extends StatelessWidget {
  final dynamic assignee;

  const _AssigneeChip({required this.assignee});

  @override
  Widget build(BuildContext context) {
    return Chip(
      avatar: CircleAvatar(
        backgroundImage: assignee.user?.avatarUrl != null
            ? NetworkImage(assignee.user!.avatarUrl!)
            : null,
        child: assignee.user?.avatarUrl == null
            ? Text(
                assignee.user?.displayName[0].toUpperCase() ?? 'U',
                style: const TextStyle(fontSize: 12),
              )
            : null,
      ),
      label: Text(assignee.user?.displayName ?? 'Unknown'),
    );
  }
}

class _MergeRequestItem extends StatelessWidget {
  final dynamic mr;

  const _MergeRequestItem({required this.mr});

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        leading: Icon(
          Icons.merge,
          color: mr.state == 'merged'
              ? AppTheme.successColor
              : mr.state == 'closed'
                  ? AppTheme.errorColor
                  : AppTheme.primaryColor,
        ),
        title: Text(mr.title ?? '!${mr.mrIid}'),
        subtitle: Text('${mr.sourceBranch} → ${mr.targetBranch}'),
        trailing: _MRStateBadge(state: mr.state),
        onTap: () async {
          final uri = Uri.parse(mr.mrUrl);
          if (await canLaunchUrl(uri)) {
            await launchUrl(uri, mode: LaunchMode.externalApplication);
          }
        },
      ),
    );
  }
}

class _MRStateBadge extends StatelessWidget {
  final String state;

  const _MRStateBadge({required this.state});

  @override
  Widget build(BuildContext context) {
    final color = _getStateColor(state);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: color.withOpacity(0.1),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        state,
        style: Theme.of(context).textTheme.labelSmall?.copyWith(
              color: color,
            ),
      ),
    );
  }

  Color _getStateColor(String state) {
    switch (state) {
      case 'merged':
        return AppTheme.successColor;
      case 'opened':
        return AppTheme.primaryColor;
      case 'closed':
        return AppTheme.errorColor;
      default:
        return Colors.grey;
    }
  }
}

class _SubTicketItem extends StatelessWidget {
  final dynamic ticket;

  const _SubTicketItem({required this.ticket});

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        leading: Container(
          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
          decoration: BoxDecoration(
            color: AppTheme.primaryColor.withOpacity(0.1),
            borderRadius: BorderRadius.circular(4),
          ),
          child: Text(
            ticket.identifier,
            style: Theme.of(context).textTheme.labelSmall?.copyWith(
                  color: AppTheme.primaryColor,
                  fontWeight: FontWeight.bold,
                ),
          ),
        ),
        title: Text(
          ticket.title,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
        ),
        trailing: _StatusBadge(status: ticket.status),
      ),
    );
  }
}
