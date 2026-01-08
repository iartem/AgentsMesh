import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:agentmesh/providers/auth_provider.dart';
import 'package:agentmesh/providers/organization_provider.dart';
import 'package:agentmesh/utils/theme.dart';

class SettingsScreen extends ConsumerStatefulWidget {
  const SettingsScreen({super.key});

  @override
  ConsumerState<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends ConsumerState<SettingsScreen> {
  PackageInfo? _packageInfo;

  @override
  void initState() {
    super.initState();
    _loadPackageInfo();
  }

  Future<void> _loadPackageInfo() async {
    final info = await PackageInfo.fromPlatform();
    if (mounted) {
      setState(() {
        _packageInfo = info;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authProvider);
    final orgState = ref.watch(organizationProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Settings'),
      ),
      body: ListView(
        children: [
          // Profile section
          _buildSection(
            context,
            title: 'Profile',
            children: [
              ListTile(
                leading: CircleAvatar(
                  radius: 24,
                  backgroundImage: authState.user?.avatarUrl != null
                      ? NetworkImage(authState.user!.avatarUrl!)
                      : null,
                  child: authState.user?.avatarUrl == null
                      ? Text(
                          authState.user?.displayName[0].toUpperCase() ?? 'U',
                          style: const TextStyle(fontSize: 20),
                        )
                      : null,
                ),
                title: Text(authState.user?.displayName ?? 'User'),
                subtitle: Text(authState.user?.email ?? ''),
                trailing: const Icon(Icons.chevron_right),
                onTap: () {
                  // TODO: Navigate to profile edit screen
                },
              ),
            ],
          ),

          // Organization section
          _buildSection(
            context,
            title: 'Organization',
            children: [
              ListTile(
                leading: const Icon(Icons.business),
                title: const Text('Current Organization'),
                subtitle: Text(orgState.current?.name ?? 'No organization'),
                trailing: const Icon(Icons.chevron_right),
                onTap: () => _showOrganizationPicker(context),
              ),
              ListTile(
                leading: const Icon(Icons.people),
                title: const Text('Team'),
                subtitle: const Text('View team members'),
                trailing: const Icon(Icons.chevron_right),
                onTap: () {
                  // TODO: Navigate to team screen
                },
              ),
            ],
          ),

          // Notifications section
          _buildSection(
            context,
            title: 'Notifications',
            children: [
              SwitchListTile(
                secondary: const Icon(Icons.notifications),
                title: const Text('Push Notifications'),
                subtitle: const Text('Receive push notifications'),
                value: true,
                onChanged: (value) {
                  // TODO: Implement notification toggle
                },
              ),
              SwitchListTile(
                secondary: const Icon(Icons.email),
                title: const Text('Email Notifications'),
                subtitle: const Text('Receive email updates'),
                value: true,
                onChanged: (value) {
                  // TODO: Implement email notification toggle
                },
              ),
            ],
          ),

          // Appearance section
          _buildSection(
            context,
            title: 'Appearance',
            children: [
              ListTile(
                leading: const Icon(Icons.dark_mode),
                title: const Text('Theme'),
                subtitle: const Text('System default'),
                trailing: const Icon(Icons.chevron_right),
                onTap: () => _showThemePicker(context),
              ),
            ],
          ),

          // About section
          _buildSection(
            context,
            title: 'About',
            children: [
              ListTile(
                leading: const Icon(Icons.info),
                title: const Text('Version'),
                subtitle: Text(_packageInfo != null
                    ? '${_packageInfo!.version} (${_packageInfo!.buildNumber})'
                    : 'Loading...'),
              ),
              ListTile(
                leading: const Icon(Icons.description),
                title: const Text('Terms of Service'),
                trailing: const Icon(Icons.open_in_new),
                onTap: () {
                  // TODO: Open terms URL
                },
              ),
              ListTile(
                leading: const Icon(Icons.privacy_tip),
                title: const Text('Privacy Policy'),
                trailing: const Icon(Icons.open_in_new),
                onTap: () {
                  // TODO: Open privacy URL
                },
              ),
              ListTile(
                leading: const Icon(Icons.help),
                title: const Text('Help & Support'),
                trailing: const Icon(Icons.open_in_new),
                onTap: () {
                  // TODO: Open help URL
                },
              ),
            ],
          ),

          // Logout
          const SizedBox(height: 16),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: OutlinedButton(
              onPressed: () => _showLogoutDialog(context),
              style: OutlinedButton.styleFrom(
                foregroundColor: AppTheme.errorColor,
                side: BorderSide(color: AppTheme.errorColor),
              ),
              child: const Text('Sign Out'),
            ),
          ),
          const SizedBox(height: 32),
        ],
      ),
    );
  }

  Widget _buildSection(
    BuildContext context, {
    required String title,
    required List<Widget> children,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 24, 16, 8),
          child: Text(
            title,
            style: Theme.of(context).textTheme.titleSmall?.copyWith(
                  color: AppTheme.primaryColor,
                  fontWeight: FontWeight.bold,
                ),
          ),
        ),
        Card(
          margin: const EdgeInsets.symmetric(horizontal: 16),
          child: Column(
            children: children,
          ),
        ),
      ],
    );
  }

  void _showOrganizationPicker(BuildContext context) {
    final orgState = ref.read(organizationProvider);

    showModalBottomSheet(
      context: context,
      builder: (context) => Container(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              'Select Organization',
              style: Theme.of(context).textTheme.titleLarge?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: 16),
            ...orgState.organizations.map((org) => ListTile(
                  leading: CircleAvatar(
                    backgroundImage:
                        org.logoUrl != null ? NetworkImage(org.logoUrl!) : null,
                    child: org.logoUrl == null
                        ? Text(org.name[0].toUpperCase())
                        : null,
                  ),
                  title: Text(org.name),
                  subtitle: Text(org.slug),
                  trailing: org.id == orgState.current?.id
                      ? const Icon(Icons.check, color: AppTheme.primaryColor)
                      : null,
                  onTap: () {
                    ref
                        .read(organizationProvider.notifier)
                        .selectOrganization(org);
                    Navigator.pop(context);
                  },
                )),
            const SizedBox(height: 16),
            OutlinedButton.icon(
              onPressed: () {
                Navigator.pop(context);
                // TODO: Navigate to create organization screen
              },
              icon: const Icon(Icons.add),
              label: const Text('Create Organization'),
            ),
          ],
        ),
      ),
    );
  }

  void _showThemePicker(BuildContext context) {
    showModalBottomSheet(
      context: context,
      builder: (context) => Container(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              'Select Theme',
              style: Theme.of(context).textTheme.titleLarge?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: 16),
            ListTile(
              leading: const Icon(Icons.settings_suggest),
              title: const Text('System Default'),
              trailing: const Icon(Icons.check, color: AppTheme.primaryColor),
              onTap: () => Navigator.pop(context),
            ),
            ListTile(
              leading: const Icon(Icons.light_mode),
              title: const Text('Light'),
              onTap: () => Navigator.pop(context),
            ),
            ListTile(
              leading: const Icon(Icons.dark_mode),
              title: const Text('Dark'),
              onTap: () => Navigator.pop(context),
            ),
          ],
        ),
      ),
    );
  }

  void _showLogoutDialog(BuildContext context) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Sign Out'),
        content: const Text('Are you sure you want to sign out?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () async {
              Navigator.pop(context);
              await ref.read(authProvider.notifier).logout();
              if (context.mounted) {
                context.go('/login');
              }
            },
            style: FilledButton.styleFrom(
              backgroundColor: AppTheme.errorColor,
            ),
            child: const Text('Sign Out'),
          ),
        ],
      ),
    );
  }
}
