import type { Action, Step } from "./workflow";
export interface StepLocation {
  step: Step;
  siblings: Step[];
  index: number;
  parent?: string;
  depth: number;
  actionsOnly: boolean;
}
export function locateStep(
  steps: Step[],
  id: string,
  parent?: string,
  depth = 0,
  actionsOnly = false,
): StepLocation | undefined {
  for (let index = 0; index < steps.length; index++) {
    const step = steps[index];
    if (step.id === id)
      return { step, siblings: steps, index, parent, depth, actionsOnly };
    const found =
      step.kind === "parallel"
        ? locateStep(step.actions, id, step.id, depth, true)
        : step.kind === "wait_reply"
          ? locateStep(step.timeoutSteps ?? [], id, step.id, depth + 1)
          : undefined;
    if (found) return found;
  }
}
export function replaceSequence(
  steps: Step[],
  parent: string | undefined,
  replacement: Step[],
): Step[] {
  if (!parent) return replacement;
  return steps.map((step) => {
    if (step.kind === "parallel")
      return step.id === parent
        ? { ...step, actions: replacement as Action[] }
        : step;
    if (
      step.kind === "wait_reply" &&
      (step.id === parent ||
        (step.timeoutSteps && locateStep(step.timeoutSteps, parent)))
    )
      return {
        ...step,
        timeoutSteps:
          step.id === parent
            ? replacement
            : replaceSequence(step.timeoutSteps ?? [], parent, replacement),
      };
    return step;
  });
}
export function replaceStep(steps: Step[], replacement: Step): Step[] {
  const location = locateStep(steps, replacement.id);
  if (!location) return steps;
  return replaceSequence(
    steps,
    location.parent,
    location.siblings.map((s) => (s.id === replacement.id ? replacement : s)),
  );
}
export function cloneStep(step: Step): Step {
  const clone = { ...structuredClone(step), id: crypto.randomUUID() };
  if (clone.kind === "parallel")
    clone.actions = clone.actions.map((a) => cloneStep(a) as Action);
  if (clone.kind === "wait_reply" && clone.timeoutSteps)
    clone.timeoutSteps = clone.timeoutSteps.map(cloneStep);
  return clone;
}
export function moveStep(steps: Step[], id: string, toIndex: number): Step[] {
  const location = locateStep(steps, id);
  if (
    !location ||
    toIndex < 0 ||
    toIndex >= location.siblings.length ||
    toIndex === location.index
  )
    return steps;
  const siblings = [...location.siblings];
  siblings.splice(location.index, 1);
  siblings.splice(toIndex, 0, location.step);
  return replaceSequence(steps, location.parent, siblings);
}
