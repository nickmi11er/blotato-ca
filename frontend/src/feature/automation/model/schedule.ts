// datetime-local uses the browser's timezone. Preserve a concrete instant and
// reject invalid dates / spring-forward gaps instead of silently shifting time.
export function localDateValue(at: string): string {
  const date = new Date(at);
  if (!Number.isFinite(date.getTime())) return "";
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000)
    .toISOString()
    .slice(0, 16);
}
export function localDateToInstant(value: string): string | null {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) return null;
  const date = new Date(value);
  if (
    !Number.isFinite(date.getTime()) ||
    localDateValue(date.toISOString()) !== value
  )
    return null;
  return date.toISOString();
}

// Find the next wall-clock occurrence in the recorded IANA zone, independent of
// the browser's zone. Searching real instants naturally handles repeated hours;
// nonexistent spring-forward times are skipped for that day.
export function nextTimeInstant(
  time: string,
  timezone: string,
  now: number,
): number | null {
  if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(time)) return null;
  try {
    const formatter = new Intl.DateTimeFormat("en-GB", {
      timeZone: timezone,
      hour: "2-digit",
      minute: "2-digit",
      hourCycle: "h23",
    });
    const start = Math.ceil(now / 60000) * 60000;
    for (let minute = 0; minute < 60 * 72; minute++) {
      const instant = start + minute * 60000;
      if (formatter.format(instant) === time) return instant;
    }
  } catch {
    return null;
  }
  return null;
}
