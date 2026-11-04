export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}
export async function request<T = void>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const response = await fetch(`/api/${path}`, {
    ...options,
    headers: { "Content-Type": "application/json", ...options.headers },
  });
  if (!response.ok) {
    const text = (await response.text()).trim();
    throw new ApiError(
      response.status,
      response.status >= 500
        ? "The server is unavailable. Your edits are still here. Please try again."
        : text || `Request failed (${response.status}).`,
    );
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}
export function errorMessage(error: unknown): string {
  if (error instanceof ApiError) return error.message;
  return "Could not reach the server. Your edits are still here. Please try again.";
}
export const versionHeader = (version: number) => ({
  "If-Match": `"${version}"`,
});
