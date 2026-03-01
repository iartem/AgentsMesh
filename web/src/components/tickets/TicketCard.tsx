"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuthStore } from "@/stores/auth";
import { Ticket } from "@/stores/ticket";
import { StatusIcon, PriorityIcon, getStatusDisplayInfo } from "./TicketIcons";

interface TicketCardProps {
  ticket: Ticket;
  onClick?: () => void;
  showRepository?: boolean;
  showStatus?: boolean;
}

export function TicketCard({ ticket, onClick, showRepository = true, showStatus = true }: TicketCardProps) {
  const t = useTranslations();
  const { currentOrg } = useAuthStore();
  const statusInfo = getStatusDisplayInfo(ticket.status);

  const isDueSoon = () => {
    if (!ticket.due_date) return false;
    const due = new Date(ticket.due_date);
    const now = new Date();
    const diff = due.getTime() - now.getTime();
    const days = diff / (1000 * 60 * 60 * 24);
    return days >= 0 && days <= 3;
  };

  const isOverdue = () => {
    if (!ticket.due_date) return false;
    const due = new Date(ticket.due_date);
    const now = new Date();
    return due < now && ticket.status !== "done";
  };

  return (
    <div
      className="border border-border/50 rounded-xl p-3.5 bg-card hover:shadow-md hover:border-primary/25 transition-all duration-200 cursor-pointer group"
      onClick={onClick}
    >
      {/* Header */}
      <div className="flex items-start justify-between gap-2 mb-2">
        <Link
          href={`/${currentOrg?.slug}/tickets/${ticket.slug}`}
          className="text-[11px] text-muted-foreground/60 hover:text-primary font-mono tracking-wide"
          onClick={(e) => e.stopPropagation()}
        >
          {ticket.slug}
        </Link>
        {showStatus && (
          <span
            className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-medium ring-1 ring-inset ring-current/10 ${statusInfo.bgColor} ${statusInfo.color}`}
          >
            <StatusIcon status={ticket.status} size="xs" />
            {t(`tickets.status.${ticket.status}`)}
          </span>
        )}
      </div>

      {/* Title */}
      <h3 className="font-semibold text-sm line-clamp-2 mb-2 group-hover:text-foreground transition-colors">{ticket.title}</h3>

      {/* Labels */}
      {ticket.labels && ticket.labels.length > 0 && (
        <div className="flex flex-wrap gap-1 mb-2">
          {ticket.labels.map((label) => (
            <span
              key={label.id}
              className="px-2 py-0.5 rounded-md text-[11px] font-medium"
              style={{
                backgroundColor: `${label.color}15`,
                color: label.color,
              }}
            >
              {label.name}
            </span>
          ))}
        </div>
      )}

      {/* Footer */}
      <div className="flex items-center justify-between mt-2.5 pt-2 border-t border-border/30">
        <div className="flex items-center gap-2">
          <PriorityIcon priority={ticket.priority} size="sm" />
          {ticket.due_date && (
            <span
              className={`text-[11px] tabular-nums ${
                isOverdue()
                  ? "text-red-600 dark:text-red-400 font-medium"
                  : isDueSoon()
                  ? "text-orange-600 dark:text-orange-400"
                  : "text-muted-foreground/60"
              }`}
            >
              {new Date(ticket.due_date).toLocaleDateString()}
            </span>
          )}
        </div>

        {/* Assignees */}
        <div className="flex -space-x-1.5">
          {ticket.assignees?.slice(0, 3).map((assignee) => (
            <div
              key={assignee.user_id}
              className="w-6 h-6 rounded-full border-2 border-background overflow-hidden ring-1 ring-border/20"
              title={assignee.user?.name || assignee.user?.username}
            >
              {assignee.user?.avatar_url ? (
                /* eslint-disable-next-line @next/next/no-img-element */
                <img
                  src={assignee.user.avatar_url}
                  alt={assignee.user?.username}
                  className="w-full h-full object-cover"
                />
              ) : (
                <div className="w-full h-full bg-primary/10 flex items-center justify-center text-[10px] font-semibold text-primary">
                  {(assignee.user?.name || assignee.user?.username || "?")[0].toUpperCase()}
                </div>
              )}
            </div>
          ))}
          {ticket.assignees && ticket.assignees.length > 3 && (
            <div className="w-6 h-6 rounded-full border-2 border-background bg-muted flex items-center justify-center text-[10px] font-medium text-muted-foreground">
              +{ticket.assignees.length - 3}
            </div>
          )}
        </div>
      </div>

      {/* Repository */}
      {showRepository && ticket.repository && (
        <div className="mt-2 text-[11px] text-muted-foreground/50 font-mono">
          {ticket.repository.name}
        </div>
      )}
    </div>
  );
}

export default TicketCard;
