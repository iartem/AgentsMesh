import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:agentmesh/models/user.dart';
import 'package:agentmesh/providers/auth_provider.dart';
import 'package:agentmesh/utils/constants.dart';

class OrganizationState {
  final List<Organization> organizations;
  final Organization? current;
  final bool isLoading;
  final String? error;

  const OrganizationState({
    this.organizations = const [],
    this.current,
    this.isLoading = false,
    this.error,
  });

  OrganizationState copyWith({
    List<Organization>? organizations,
    Organization? current,
    bool? isLoading,
    String? error,
  }) {
    return OrganizationState(
      organizations: organizations ?? this.organizations,
      current: current ?? this.current,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}

class OrganizationNotifier extends StateNotifier<OrganizationState> {
  final Ref _ref;
  final FlutterSecureStorage _storage = const FlutterSecureStorage();

  OrganizationNotifier(this._ref) : super(const OrganizationState());

  Future<void> loadOrganizations() async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final apiClient = _ref.read(apiClientProvider);
      final response = await apiClient.getUserOrganizations();
      final List<dynamic> orgList = response.data['organizations'] ?? [];
      final organizations =
          orgList.map((o) => Organization.fromJson(o)).toList();

      Organization? current;
      final savedSlug = await _storage.read(key: StorageKeys.currentOrganization);

      if (savedSlug != null) {
        current = organizations.firstWhere(
          (o) => o.slug == savedSlug,
          orElse: () => organizations.first,
        );
      } else if (organizations.isNotEmpty) {
        current = organizations.first;
      }

      if (current != null) {
        apiClient.setOrganization(current.slug);
        await _storage.write(
            key: StorageKeys.currentOrganization, value: current.slug);
      }

      state = state.copyWith(
        organizations: organizations,
        current: current,
        isLoading: false,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: 'Failed to load organizations',
      );
    }
  }

  Future<void> selectOrganization(Organization org) async {
    final apiClient = _ref.read(apiClientProvider);
    apiClient.setOrganization(org.slug);
    await _storage.write(key: StorageKeys.currentOrganization, value: org.slug);
    state = state.copyWith(current: org);
  }

  void clear() {
    state = const OrganizationState();
  }
}

final organizationProvider =
    StateNotifierProvider<OrganizationNotifier, OrganizationState>((ref) {
  return OrganizationNotifier(ref);
});

final currentOrganizationProvider = Provider<Organization?>((ref) {
  return ref.watch(organizationProvider).current;
});
