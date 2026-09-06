import { Toast } from "@base-ui/react/toast";

export const toastManager = Toast.createToastManager();

export function showToast(title: string, opts?: { description?: string; type?: string }) {
  toastManager.add({
    title,
    description: opts?.description,
    type: opts?.type ?? "default",
  });
}
