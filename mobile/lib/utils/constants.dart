class ApiConstants {
  static const String baseUrl = 'https://api.agentmesh.dev';
  static const Duration connectTimeout = Duration(seconds: 30);
  static const Duration receiveTimeout = Duration(seconds: 30);
}

class StorageKeys {
  static const String accessToken = 'access_token';
  static const String refreshToken = 'refresh_token';
  static const String currentOrganization = 'current_organization';
  static const String userProfile = 'user_profile';
}

class TicketStatus {
  static const String backlog = 'backlog';
  static const String todo = 'todo';
  static const String inProgress = 'in_progress';
  static const String inReview = 'in_review';
  static const String done = 'done';
  static const String cancelled = 'cancelled';

  static String displayName(String status) {
    switch (status) {
      case backlog:
        return 'Backlog';
      case todo:
        return 'To Do';
      case inProgress:
        return 'In Progress';
      case inReview:
        return 'In Review';
      case done:
        return 'Done';
      case cancelled:
        return 'Cancelled';
      default:
        return status;
    }
  }
}

class TicketPriority {
  static const String none = 'none';
  static const String low = 'low';
  static const String medium = 'medium';
  static const String high = 'high';
  static const String urgent = 'urgent';

  static String displayName(String priority) {
    switch (priority) {
      case none:
        return 'None';
      case low:
        return 'Low';
      case medium:
        return 'Medium';
      case high:
        return 'High';
      case urgent:
        return 'Urgent';
      default:
        return priority;
    }
  }
}

class SessionStatus {
  static const String initializing = 'initializing';
  static const String ready = 'ready';
  static const String running = 'running';
  static const String paused = 'paused';
  static const String terminated = 'terminated';
  static const String error = 'error';

  static String displayName(String status) {
    switch (status) {
      case initializing:
        return 'Initializing';
      case ready:
        return 'Ready';
      case running:
        return 'Running';
      case paused:
        return 'Paused';
      case terminated:
        return 'Terminated';
      case error:
        return 'Error';
      default:
        return status;
    }
  }
}

class RunnerStatus {
  static const String online = 'online';
  static const String offline = 'offline';
  static const String busy = 'busy';
  static const String maintenance = 'maintenance';

  static String displayName(String status) {
    switch (status) {
      case online:
        return 'Online';
      case offline:
        return 'Offline';
      case busy:
        return 'Busy';
      case maintenance:
        return 'Maintenance';
      default:
        return status;
    }
  }
}
