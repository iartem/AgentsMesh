import 'package:json_annotation/json_annotation.dart';
import 'user.dart';

part 'ticket.g.dart';

@JsonSerializable()
class Ticket {
  final int id;
  @JsonKey(name: 'organization_id')
  final int organizationId;
  @JsonKey(name: 'team_id')
  final int? teamId;
  final int number;
  final String identifier;
  final String type;
  final String title;
  final String? description;
  final String? content;
  final String status;
  final String priority;
  @JsonKey(name: 'due_date')
  final DateTime? dueDate;
  @JsonKey(name: 'started_at')
  final DateTime? startedAt;
  @JsonKey(name: 'completed_at')
  final DateTime? completedAt;
  @JsonKey(name: 'repository_id')
  final int? repositoryId;
  @JsonKey(name: 'reporter_id')
  final int reporterId;
  @JsonKey(name: 'parent_ticket_id')
  final int? parentTicketId;
  @JsonKey(name: 'created_at')
  final DateTime createdAt;
  @JsonKey(name: 'updated_at')
  final DateTime updatedAt;

  final List<TicketAssignee>? assignees;
  final List<Label>? labels;
  @JsonKey(name: 'merge_requests')
  final List<MergeRequest>? mergeRequests;
  @JsonKey(name: 'sub_tickets')
  final List<Ticket>? subTickets;
  final User? reporter;

  Ticket({
    required this.id,
    required this.organizationId,
    this.teamId,
    required this.number,
    required this.identifier,
    required this.type,
    required this.title,
    this.description,
    this.content,
    required this.status,
    required this.priority,
    this.dueDate,
    this.startedAt,
    this.completedAt,
    this.repositoryId,
    required this.reporterId,
    this.parentTicketId,
    required this.createdAt,
    required this.updatedAt,
    this.assignees,
    this.labels,
    this.mergeRequests,
    this.subTickets,
    this.reporter,
  });

  factory Ticket.fromJson(Map<String, dynamic> json) => _$TicketFromJson(json);
  Map<String, dynamic> toJson() => _$TicketToJson(this);

  bool get isOverdue {
    if (dueDate == null) return false;
    if (status == 'done' || status == 'cancelled') return false;
    return dueDate!.isBefore(DateTime.now());
  }

  bool get isCompleted => status == 'done' || status == 'cancelled';
}

@JsonSerializable()
class TicketAssignee {
  final int id;
  @JsonKey(name: 'ticket_id')
  final int ticketId;
  @JsonKey(name: 'user_id')
  final int userId;
  final User? user;

  TicketAssignee({
    required this.id,
    required this.ticketId,
    required this.userId,
    this.user,
  });

  factory TicketAssignee.fromJson(Map<String, dynamic> json) =>
      _$TicketAssigneeFromJson(json);
  Map<String, dynamic> toJson() => _$TicketAssigneeToJson(this);
}

@JsonSerializable()
class Label {
  final int id;
  @JsonKey(name: 'organization_id')
  final int organizationId;
  @JsonKey(name: 'repository_id')
  final int? repositoryId;
  final String name;
  final String color;

  Label({
    required this.id,
    required this.organizationId,
    this.repositoryId,
    required this.name,
    required this.color,
  });

  factory Label.fromJson(Map<String, dynamic> json) => _$LabelFromJson(json);
  Map<String, dynamic> toJson() => _$LabelToJson(this);
}

@JsonSerializable()
class MergeRequest {
  final int id;
  @JsonKey(name: 'organization_id')
  final int organizationId;
  @JsonKey(name: 'ticket_id')
  final int ticketId;
  @JsonKey(name: 'session_id')
  final int? sessionId;
  @JsonKey(name: 'mr_iid')
  final int mrIid;
  @JsonKey(name: 'mr_url')
  final String mrUrl;
  @JsonKey(name: 'source_branch')
  final String sourceBranch;
  @JsonKey(name: 'target_branch')
  final String targetBranch;
  final String? title;
  final String state;
  @JsonKey(name: 'created_at')
  final DateTime createdAt;

  MergeRequest({
    required this.id,
    required this.organizationId,
    required this.ticketId,
    this.sessionId,
    required this.mrIid,
    required this.mrUrl,
    required this.sourceBranch,
    required this.targetBranch,
    this.title,
    required this.state,
    required this.createdAt,
  });

  factory MergeRequest.fromJson(Map<String, dynamic> json) =>
      _$MergeRequestFromJson(json);
  Map<String, dynamic> toJson() => _$MergeRequestToJson(this);
}

@JsonSerializable()
class CreateTicketRequest {
  @JsonKey(name: 'team_id')
  final int? teamId;
  @JsonKey(name: 'repository_id')
  final int? repositoryId;
  final String type;
  final String title;
  final String? description;
  final String priority;
  @JsonKey(name: 'due_date')
  final DateTime? dueDate;
  @JsonKey(name: 'assignee_ids')
  final List<int>? assigneeIds;
  @JsonKey(name: 'label_ids')
  final List<int>? labelIds;

  CreateTicketRequest({
    this.teamId,
    this.repositoryId,
    required this.type,
    required this.title,
    this.description,
    required this.priority,
    this.dueDate,
    this.assigneeIds,
    this.labelIds,
  });

  factory CreateTicketRequest.fromJson(Map<String, dynamic> json) =>
      _$CreateTicketRequestFromJson(json);
  Map<String, dynamic> toJson() => _$CreateTicketRequestToJson(this);
}

@JsonSerializable()
class UpdateTicketRequest {
  final String? title;
  final String? description;
  final String? type;
  final String? status;
  final String? priority;
  @JsonKey(name: 'due_date')
  final DateTime? dueDate;
  @JsonKey(name: 'assignee_ids')
  final List<int>? assigneeIds;
  @JsonKey(name: 'label_ids')
  final List<int>? labelIds;

  UpdateTicketRequest({
    this.title,
    this.description,
    this.type,
    this.status,
    this.priority,
    this.dueDate,
    this.assigneeIds,
    this.labelIds,
  });

  factory UpdateTicketRequest.fromJson(Map<String, dynamic> json) =>
      _$UpdateTicketRequestFromJson(json);
  Map<String, dynamic> toJson() => _$UpdateTicketRequestToJson(this);
}
