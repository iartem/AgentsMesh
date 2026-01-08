import 'package:json_annotation/json_annotation.dart';

part 'user.g.dart';

@JsonSerializable()
class User {
  final int id;
  final String email;
  final String username;
  final String? name;
  @JsonKey(name: 'avatar_url')
  final String? avatarUrl;
  @JsonKey(name: 'is_active')
  final bool isActive;
  @JsonKey(name: 'last_login_at')
  final DateTime? lastLoginAt;
  @JsonKey(name: 'created_at')
  final DateTime createdAt;

  User({
    required this.id,
    required this.email,
    required this.username,
    this.name,
    this.avatarUrl,
    required this.isActive,
    this.lastLoginAt,
    required this.createdAt,
  });

  factory User.fromJson(Map<String, dynamic> json) => _$UserFromJson(json);
  Map<String, dynamic> toJson() => _$UserToJson(this);

  String get displayName => name ?? username;
}

@JsonSerializable()
class Organization {
  final int id;
  final String name;
  final String slug;
  @JsonKey(name: 'logo_url')
  final String? logoUrl;
  @JsonKey(name: 'subscription_plan')
  final String subscriptionPlan;
  @JsonKey(name: 'subscription_status')
  final String subscriptionStatus;
  @JsonKey(name: 'created_at')
  final DateTime createdAt;

  Organization({
    required this.id,
    required this.name,
    required this.slug,
    this.logoUrl,
    required this.subscriptionPlan,
    required this.subscriptionStatus,
    required this.createdAt,
  });

  factory Organization.fromJson(Map<String, dynamic> json) =>
      _$OrganizationFromJson(json);
  Map<String, dynamic> toJson() => _$OrganizationToJson(this);
}

@JsonSerializable()
class OrganizationMember {
  final int id;
  @JsonKey(name: 'organization_id')
  final int organizationId;
  @JsonKey(name: 'user_id')
  final int userId;
  final String role;
  @JsonKey(name: 'joined_at')
  final DateTime joinedAt;
  final User? user;

  OrganizationMember({
    required this.id,
    required this.organizationId,
    required this.userId,
    required this.role,
    required this.joinedAt,
    this.user,
  });

  factory OrganizationMember.fromJson(Map<String, dynamic> json) =>
      _$OrganizationMemberFromJson(json);
  Map<String, dynamic> toJson() => _$OrganizationMemberToJson(this);
}

@JsonSerializable()
class Team {
  final int id;
  @JsonKey(name: 'organization_id')
  final int organizationId;
  final String name;
  final String? description;
  @JsonKey(name: 'created_at')
  final DateTime createdAt;

  Team({
    required this.id,
    required this.organizationId,
    required this.name,
    this.description,
    required this.createdAt,
  });

  factory Team.fromJson(Map<String, dynamic> json) => _$TeamFromJson(json);
  Map<String, dynamic> toJson() => _$TeamToJson(this);
}
