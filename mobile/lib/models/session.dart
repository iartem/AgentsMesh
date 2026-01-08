import 'package:json_annotation/json_annotation.dart';
import 'user.dart';

part 'session.g.dart';

@JsonSerializable()
class Session {
  final int id;
  @JsonKey(name: 'organization_id')
  final int organizationId;
  @JsonKey(name: 'team_id')
  final int? teamId;
  @JsonKey(name: 'session_key')
  final String sessionKey;
  @JsonKey(name: 'runner_id')
  final int runnerId;
  @JsonKey(name: 'agent_type_id')
  final int? agentTypeId;
  @JsonKey(name: 'repository_id')
  final int? repositoryId;
  @JsonKey(name: 'ticket_id')
  final int? ticketId;
  @JsonKey(name: 'created_by_id')
  final int createdById;
  @JsonKey(name: 'pty_pid')
  final int? ptyPid;
  final String status;
  @JsonKey(name: 'agent_status')
  final String agentStatus;
  @JsonKey(name: 'started_at')
  final DateTime? startedAt;
  @JsonKey(name: 'finished_at')
  final DateTime? finishedAt;
  @JsonKey(name: 'last_activity')
  final DateTime? lastActivity;
  @JsonKey(name: 'initial_prompt')
  final String? initialPrompt;
  @JsonKey(name: 'branch_name')
  final String? branchName;
  @JsonKey(name: 'worktree_path')
  final String? worktreePath;
  @JsonKey(name: 'created_at')
  final DateTime createdAt;
  @JsonKey(name: 'updated_at')
  final DateTime updatedAt;

  final Runner? runner;
  final AgentType? agentType;
  final User? createdBy;

  Session({
    required this.id,
    required this.organizationId,
    this.teamId,
    required this.sessionKey,
    required this.runnerId,
    this.agentTypeId,
    this.repositoryId,
    this.ticketId,
    required this.createdById,
    this.ptyPid,
    required this.status,
    required this.agentStatus,
    this.startedAt,
    this.finishedAt,
    this.lastActivity,
    this.initialPrompt,
    this.branchName,
    this.worktreePath,
    required this.createdAt,
    required this.updatedAt,
    this.runner,
    this.agentType,
    this.createdBy,
  });

  factory Session.fromJson(Map<String, dynamic> json) =>
      _$SessionFromJson(json);
  Map<String, dynamic> toJson() => _$SessionToJson(this);

  bool get isActive => status == 'running' || status == 'ready';
  bool get isTerminated => status == 'terminated' || status == 'error';

  Duration? get duration {
    if (startedAt == null) return null;
    final end = finishedAt ?? DateTime.now();
    return end.difference(startedAt!);
  }
}

@JsonSerializable()
class Runner {
  final int id;
  @JsonKey(name: 'organization_id')
  final int organizationId;
  @JsonKey(name: 'node_id')
  final String nodeId;
  final String? description;
  final String status;
  @JsonKey(name: 'last_heartbeat')
  final DateTime? lastHeartbeat;
  @JsonKey(name: 'current_sessions')
  final int currentSessions;
  @JsonKey(name: 'max_concurrent_sessions')
  final int maxConcurrentSessions;
  @JsonKey(name: 'runner_version')
  final String? runnerVersion;
  @JsonKey(name: 'host_info')
  final Map<String, dynamic>? hostInfo;
  @JsonKey(name: 'created_at')
  final DateTime createdAt;

  Runner({
    required this.id,
    required this.organizationId,
    required this.nodeId,
    this.description,
    required this.status,
    this.lastHeartbeat,
    required this.currentSessions,
    required this.maxConcurrentSessions,
    this.runnerVersion,
    this.hostInfo,
    required this.createdAt,
  });

  factory Runner.fromJson(Map<String, dynamic> json) => _$RunnerFromJson(json);
  Map<String, dynamic> toJson() => _$RunnerToJson(this);

  bool get isOnline => status == 'online';
  bool get hasCapacity => currentSessions < maxConcurrentSessions;

  String? get os => hostInfo?['os'] as String?;
  String? get arch => hostInfo?['arch'] as String?;
}

@JsonSerializable()
class AgentType {
  final int id;
  final String slug;
  final String name;
  final String? description;
  @JsonKey(name: 'launch_command')
  final String launchCommand;
  @JsonKey(name: 'default_args')
  final String? defaultArgs;
  @JsonKey(name: 'is_builtin')
  final bool isBuiltin;
  @JsonKey(name: 'is_active')
  final bool isActive;

  AgentType({
    required this.id,
    required this.slug,
    required this.name,
    this.description,
    required this.launchCommand,
    this.defaultArgs,
    required this.isBuiltin,
    required this.isActive,
  });

  factory AgentType.fromJson(Map<String, dynamic> json) =>
      _$AgentTypeFromJson(json);
  Map<String, dynamic> toJson() => _$AgentTypeToJson(this);
}

@JsonSerializable()
class CreateSessionRequest {
  @JsonKey(name: 'runner_id')
  final int runnerId;
  @JsonKey(name: 'agent_type_id')
  final int? agentTypeId;
  @JsonKey(name: 'repository_id')
  final int? repositoryId;
  @JsonKey(name: 'ticket_id')
  final int? ticketId;
  @JsonKey(name: 'initial_prompt')
  final String? initialPrompt;

  CreateSessionRequest({
    required this.runnerId,
    this.agentTypeId,
    this.repositoryId,
    this.ticketId,
    this.initialPrompt,
  });

  factory CreateSessionRequest.fromJson(Map<String, dynamic> json) =>
      _$CreateSessionRequestFromJson(json);
  Map<String, dynamic> toJson() => _$CreateSessionRequestToJson(this);
}
