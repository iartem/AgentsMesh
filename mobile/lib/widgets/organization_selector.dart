import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:agentmesh/providers/organization_provider.dart';
import 'package:agentmesh/utils/theme.dart';

class OrganizationSelector extends ConsumerWidget {
  const OrganizationSelector({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final orgState = ref.watch(organizationProvider);
    final current = orgState.current;

    if (orgState.isLoading) {
      return const Card(
        child: Padding(
          padding: EdgeInsets.all(16),
          child: Center(child: CircularProgressIndicator()),
        ),
      );
    }

    if (current == null) {
      return Card(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            children: [
              const Icon(Icons.business, size: 48, color: Colors.grey),
              const SizedBox(height: 8),
              Text(
                'No organization selected',
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                      color: Colors.grey,
                    ),
              ),
              const SizedBox(height: 12),
              FilledButton.icon(
                onPressed: () {
                  // TODO: Create organization
                },
                icon: const Icon(Icons.add),
                label: const Text('Create Organization'),
              ),
            ],
          ),
        ),
      );
    }

    return Card(
      child: InkWell(
        onTap: () => _showOrganizationPicker(context, ref, orgState),
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Row(
            children: [
              CircleAvatar(
                radius: 24,
                backgroundImage: current.logoUrl != null
                    ? NetworkImage(current.logoUrl!)
                    : null,
                child: current.logoUrl == null
                    ? Text(
                        current.name[0].toUpperCase(),
                        style: const TextStyle(
                          fontSize: 20,
                          fontWeight: FontWeight.bold,
                        ),
                      )
                    : null,
              ),
              const SizedBox(width: 16),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      current.name,
                      style: Theme.of(context).textTheme.titleMedium?.copyWith(
                            fontWeight: FontWeight.bold,
                          ),
                    ),
                    const SizedBox(height: 2),
                    Row(
                      children: [
                        Container(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 6, vertical: 2),
                          decoration: BoxDecoration(
                            color: AppTheme.primaryColor.withOpacity(0.1),
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Text(
                            current.subscriptionPlan.toUpperCase(),
                            style:
                                Theme.of(context).textTheme.labelSmall?.copyWith(
                                      color: AppTheme.primaryColor,
                                      fontWeight: FontWeight.w600,
                                    ),
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              const Icon(Icons.unfold_more, color: Colors.grey),
            ],
          ),
        ),
      ),
    );
  }

  void _showOrganizationPicker(
      BuildContext context, WidgetRef ref, OrganizationState orgState) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (context) => DraggableScrollableSheet(
        initialChildSize: 0.5,
        minChildSize: 0.3,
        maxChildSize: 0.8,
        expand: false,
        builder: (context, scrollController) => Container(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              // Handle bar
              Center(
                child: Container(
                  width: 40,
                  height: 4,
                  decoration: BoxDecoration(
                    color: Colors.grey[300],
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              const SizedBox(height: 16),
              Text(
                'Switch Organization',
                style: Theme.of(context).textTheme.titleLarge?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
              ),
              const SizedBox(height: 16),
              Expanded(
                child: ListView.builder(
                  controller: scrollController,
                  itemCount: orgState.organizations.length,
                  itemBuilder: (context, index) {
                    final org = orgState.organizations[index];
                    final isSelected = org.id == orgState.current?.id;

                    return Card(
                      margin: const EdgeInsets.only(bottom: 8),
                      color: isSelected
                          ? AppTheme.primaryColor.withOpacity(0.1)
                          : null,
                      child: ListTile(
                        leading: CircleAvatar(
                          backgroundImage: org.logoUrl != null
                              ? NetworkImage(org.logoUrl!)
                              : null,
                          child: org.logoUrl == null
                              ? Text(org.name[0].toUpperCase())
                              : null,
                        ),
                        title: Text(org.name),
                        subtitle: Text(org.slug),
                        trailing: isSelected
                            ? const Icon(Icons.check,
                                color: AppTheme.primaryColor)
                            : null,
                        onTap: () {
                          ref
                              .read(organizationProvider.notifier)
                              .selectOrganization(org);
                          Navigator.pop(context);
                        },
                      ),
                    );
                  },
                ),
              ),
              const SizedBox(height: 16),
              OutlinedButton.icon(
                onPressed: () {
                  Navigator.pop(context);
                  // TODO: Navigate to create organization screen
                },
                icon: const Icon(Icons.add),
                label: const Text('Create New Organization'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
