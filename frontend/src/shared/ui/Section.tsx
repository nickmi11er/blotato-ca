import type { ReactNode } from "react";
import { Icon, type IconName } from "./Icon";
export function Section({
  number,
  title,
  subtitle,
  icon,
  children,
}: {
  number: string;
  title: string;
  subtitle: string;
  icon: IconName;
  children: ReactNode;
}) {
  return (
    <section className="editor-card" aria-labelledby={`section-${number}`}>
      <div className="section-heading">
        <span className="section-number">{number}</span>
        <div>
          <h2 id={`section-${number}`}>{title}</h2>
          <p>{subtitle}</p>
        </div>
        <span className="section-icon">
          <Icon name={icon} />
        </span>
      </div>
      <div className="section-body">{children}</div>
    </section>
  );
}
