import {
  type LucideIcon,
  Activity,
  ArrowRight,
  ArrowLeft,
  ArrowUpRight,
  Bookmark,
  Check,
  ChevronDown,
  Clock3,
  ExternalLink,
  Info,
  LayoutGrid,
  Link,
  Mail,
  MessageCircle,
  MoreHorizontal,
  Pencil,
  Play,
  Plus,
  Send,
  SlidersHorizontal,
  Sparkles,
  Users,
  X,
  Zap,
} from "lucide-react";

const icons = {
  activity: Activity,
  arrow: ArrowRight,
  arrowLeft: ArrowLeft,
  arrowUp: ArrowUpRight,
  save: Bookmark,
  check: Check,
  chevron: ChevronDown,
  clock: Clock3,
  external: ExternalLink,
  info: Info,
  grid: LayoutGrid,
  link: Link,
  mail: Mail,
  comment: MessageCircle,
  more: MoreHorizontal,
  edit: Pencil,
  play: Play,
  plus: Plus,
  send: Send,
  settings: SlidersHorizontal,
  spark: Sparkles,
  users: Users,
  close: X,
  bolt: Zap,
} satisfies Record<string, LucideIcon>;

export type IconName = keyof typeof icons;

export function Icon({
  name,
  className = "",
}: {
  name: IconName;
  className?: string;
}) {
  const Component = icons[name];
  return (
    <Component
      className={`icon ${className}`}
      strokeWidth={1.65}
      aria-hidden="true"
    />
  );
}
