import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:agentmesh/providers/ticket_provider.dart';
import 'package:agentmesh/utils/constants.dart';
import 'package:agentmesh/utils/theme.dart';

class TicketListScreen extends ConsumerStatefulWidget {
  const TicketListScreen({super.key});

  @override
  ConsumerState<TicketListScreen> createState() => _TicketListScreenState();
}

class _TicketListScreenState extends ConsumerState<TicketListScreen> {
  final _scrollController = ScrollController();
  final _searchController = TextEditingController();

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(ticketListProvider.notifier).loadTickets();
    });
  }

  @override
  void dispose() {
    _scrollController.dispose();
    _searchController.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 200) {
      final state = ref.read(ticketListProvider);
      if (!state.isLoading && state.hasMore) {
        ref.read(ticketListProvider.notifier).loadTickets();
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final ticketState = ref.watch(ticketListProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Tickets'),
        actions: [
          IconButton(
            icon: const Icon(Icons.filter_list),
            onPressed: () => _showFilterSheet(context),
          ),
        ],
      ),
      body: Column(
        children: [
          // Search bar
          Padding(
            padding: const EdgeInsets.all(16),
            child: TextField(
              controller: _searchController,
              decoration: InputDecoration(
                hintText: 'Search tickets...',
                prefixIcon: const Icon(Icons.search),
                suffixIcon: _searchController.text.isNotEmpty
                    ? IconButton(
                        icon: const Icon(Icons.clear),
                        onPressed: () {
                          _searchController.clear();
                          ref.read(ticketListProvider.notifier).setFilter(
                                ticketState.filter.copyWith(query: null),
                              );
                        },
                      )
                    : null,
              ),
              onSubmitted: (value) {
                ref.read(ticketListProvider.notifier).setFilter(
                      ticketState.filter.copyWith(
                        query: value.isEmpty ? null : value,
                      ),
                    );
              },
            ),
          ),

          // Active filters
          if (_hasActiveFilters(ticketState.filter))
            _buildActiveFilters(context, ticketState.filter),

          // Ticket list
          Expanded(
            child: RefreshIndicator(
              onRefresh: () => ref.read(ticketListProvider.notifier).refresh(),
              child: ticketState.error != null
                  ? _buildError(ticketState.error!)
                  : ticketState.tickets.isEmpty && !ticketState.isLoading
                      ? _buildEmptyState()
                      : ListView.builder(
                          controller: _scrollController,
                          padding: const EdgeInsets.symmetric(horizontal: 16),
                          itemCount: ticketState.tickets.length +
                              (ticketState.isLoading ? 1 : 0),
                          itemBuilder: (context, index) {
                            if (index >= ticketState.tickets.length) {
                              return const Center(
                                child: Padding(
                                  padding: EdgeInsets.all(16),
                                  child: CircularProgressIndicator(),
                                ),
                              );
                            }

                            final ticket = ticketState.tickets[index];
                            return _TicketListItem(
                              ticket: ticket,
                              onTap: () =>
                                  context.go('/tickets/${ticket.identifier}'),
                            );
                          },
                        ),
            ),
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: () {
          // TODO: Navigate to create ticket screen
        },
        child: const Icon(Icons.add),
      ),
    );
  }

  bool _hasActiveFilters(TicketFilter filter) {
    return filter.status != null ||
        filter.priority != null ||
        filter.assigneeId != null;
  }

  Widget _buildActiveFilters(BuildContext context, TicketFilter filter) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: Wrap(
        spacing: 8,
        runSpacing: 8,
        children: [
          if (filter.status != null)
            _FilterChip(
              label: TicketStatus.displayName(filter.status!),
              onRemove: () {
                ref.read(ticketListProvider.notifier).setFilter(
                      filter.clearFilter('status'),
                    );
              },
            ),
          if (filter.priority != null)
            _FilterChip(
              label: TicketPriority.displayName(filter.priority!),
              onRemove: () {
                ref.read(ticketListProvider.notifier).setFilter(
                      filter.clearFilter('priority'),
                    );
              },
            ),
          TextButton.icon(
            icon: const Icon(Icons.clear_all, size: 18),
            label: const Text('Clear All'),
            onPressed: () =>
                ref.read(ticketListProvider.notifier).clearFilters(),
          ),
        ],
      ),
    );
  }

  Widget _buildError(String error) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.error_outline, size: 48, color: AppTheme.errorColor),
          const SizedBox(height: 16),
          Text(error),
          const SizedBox(height: 16),
          FilledButton(
            onPressed: () => ref.read(ticketListProvider.notifier).refresh(),
            child: const Text('Retry'),
          ),
        ],
      ),
    );
  }

  Widget _buildEmptyState() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.task_outlined, size: 64, color: Colors.grey[400]),
          const SizedBox(height: 16),
          Text(
            'No tickets found',
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  color: Colors.grey,
                ),
          ),
          const SizedBox(height: 8),
          Text(
            'Create a new ticket to get started',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Colors.grey,
                ),
          ),
        ],
      ),
    );
  }

  void _showFilterSheet(BuildContext context) {
    final ticketState = ref.read(ticketListProvider);

    showModalBottomSheet(
      context: context,
      builder: (context) => _FilterSheet(
        filter: ticketState.filter,
        onApply: (filter) {
          ref.read(ticketListProvider.notifier).setFilter(filter);
          Navigator.pop(context);
        },
      ),
    );
  }
}

class _TicketListItem extends StatelessWidget {
  final dynamic ticket;
  final VoidCallback onTap;

  const _TicketListItem({
    required this.ticket,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Header row
              Row(
                children: [
                  Container(
                    padding:
                        const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                    decoration: BoxDecoration(
                      color: AppTheme.primaryColor.withOpacity(0.1),
                      borderRadius: BorderRadius.circular(4),
                    ),
                    child: Text(
                      ticket.identifier,
                      style: Theme.of(context).textTheme.labelSmall?.copyWith(
                            fontWeight: FontWeight.bold,
                            color: AppTheme.primaryColor,
                          ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  _PriorityIndicator(priority: ticket.priority),
                  const Spacer(),
                  _StatusBadge(status: ticket.status),
                ],
              ),
              const SizedBox(height: 12),

              // Title
              Text(
                ticket.title,
                style: Theme.of(context).textTheme.titleSmall?.copyWith(
                      fontWeight: FontWeight.w500,
                    ),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),

              // Description
              if (ticket.description != null) ...[
                const SizedBox(height: 4),
                Text(
                  ticket.description!,
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        color: Colors.grey,
                      ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
              const SizedBox(height: 12),

              // Footer
              Row(
                children: [
                  Icon(Icons.calendar_today, size: 14, color: Colors.grey[600]),
                  const SizedBox(width: 4),
                  Text(
                    _formatDate(ticket.createdAt),
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: Colors.grey[600],
                        ),
                  ),
                  if (ticket.isOverdue) ...[
                    const SizedBox(width: 12),
                    Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 6, vertical: 2),
                      decoration: BoxDecoration(
                        color: AppTheme.errorColor.withOpacity(0.1),
                        borderRadius: BorderRadius.circular(4),
                      ),
                      child: Text(
                        'Overdue',
                        style: Theme.of(context).textTheme.labelSmall?.copyWith(
                              color: AppTheme.errorColor,
                            ),
                      ),
                    ),
                  ],
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  String _formatDate(DateTime date) {
    final now = DateTime.now();
    final diff = now.difference(date);

    if (diff.inDays == 0) {
      return 'Today';
    } else if (diff.inDays == 1) {
      return 'Yesterday';
    } else if (diff.inDays < 7) {
      return '${diff.inDays} days ago';
    } else {
      return '${date.month}/${date.day}/${date.year}';
    }
  }
}

class _PriorityIndicator extends StatelessWidget {
  final String priority;

  const _PriorityIndicator({required this.priority});

  @override
  Widget build(BuildContext context) {
    final color = _getPriorityColor(priority);
    return Row(
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

class _StatusBadge extends StatelessWidget {
  final String status;

  const _StatusBadge({required this.status});

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
        TicketStatus.displayName(status),
        style: Theme.of(context).textTheme.labelSmall?.copyWith(
              color: color,
              fontWeight: FontWeight.w500,
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

class _FilterChip extends StatelessWidget {
  final String label;
  final VoidCallback onRemove;

  const _FilterChip({
    required this.label,
    required this.onRemove,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      decoration: BoxDecoration(
        color: AppTheme.primaryColor.withOpacity(0.1),
        borderRadius: BorderRadius.circular(16),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            label,
            style: Theme.of(context).textTheme.labelMedium?.copyWith(
                  color: AppTheme.primaryColor,
                ),
          ),
          const SizedBox(width: 4),
          GestureDetector(
            onTap: onRemove,
            child: Icon(
              Icons.close,
              size: 16,
              color: AppTheme.primaryColor,
            ),
          ),
        ],
      ),
    );
  }
}

class _FilterSheet extends StatefulWidget {
  final TicketFilter filter;
  final Function(TicketFilter) onApply;

  const _FilterSheet({
    required this.filter,
    required this.onApply,
  });

  @override
  State<_FilterSheet> createState() => _FilterSheetState();
}

class _FilterSheetState extends State<_FilterSheet> {
  late String? _status;
  late String? _priority;

  @override
  void initState() {
    super.initState();
    _status = widget.filter.status;
    _priority = widget.filter.priority;
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(24),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            'Filter Tickets',
            style: Theme.of(context).textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.bold,
                ),
          ),
          const SizedBox(height: 24),

          // Status filter
          Text(
            'Status',
            style: Theme.of(context).textTheme.titleSmall,
          ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            children: [
              for (final status in [
                'backlog',
                'todo',
                'in_progress',
                'in_review',
                'done'
              ])
                ChoiceChip(
                  label: Text(TicketStatus.displayName(status)),
                  selected: _status == status,
                  onSelected: (selected) {
                    setState(() {
                      _status = selected ? status : null;
                    });
                  },
                ),
            ],
          ),
          const SizedBox(height: 16),

          // Priority filter
          Text(
            'Priority',
            style: Theme.of(context).textTheme.titleSmall,
          ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            children: [
              for (final priority in ['urgent', 'high', 'medium', 'low', 'none'])
                ChoiceChip(
                  label: Text(TicketPriority.displayName(priority)),
                  selected: _priority == priority,
                  onSelected: (selected) {
                    setState(() {
                      _priority = selected ? priority : null;
                    });
                  },
                ),
            ],
          ),
          const SizedBox(height: 24),

          // Apply button
          FilledButton(
            onPressed: () {
              widget.onApply(widget.filter.copyWith(
                status: _status,
                priority: _priority,
              ));
            },
            child: const Text('Apply Filters'),
          ),
        ],
      ),
    );
  }
}
