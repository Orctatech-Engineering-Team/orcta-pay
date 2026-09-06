import { z } from "zod";

export const createAppSchema = z.object({
  name: z.string().min(3, "Name must be at least 3 characters"),
  product: z.string().min(2, "Product must be at least 2 characters"),
});

export type CreateAppInput = z.infer<typeof createAppSchema>;

export const settingsSchema = z.object({
  apiKey: z.string().min(10, "API key must be at least 10 characters"),
  baseUrl: z.string().url("Base URL must be a valid URL"),
});

export type SettingsInput = z.infer<typeof settingsSchema>;
