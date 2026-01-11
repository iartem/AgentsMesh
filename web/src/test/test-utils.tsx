import { ReactElement, ReactNode } from "react";
import { render, RenderOptions } from "@testing-library/react";
import { I18nProvider } from "@/lib/i18n/client";
import enMessages from "@/messages/en.json";

// Mock translations for testing
const mockTranslations = enMessages;

// Wrapper component that provides all necessary providers for testing
function AllProviders({ children }: { children: ReactNode }) {
  return (
    <I18nProvider initialLocale="en" initialTranslations={mockTranslations}>
      {children}
    </I18nProvider>
  );
}

// Custom render function that wraps components with providers
const customRender = (
  ui: ReactElement,
  options?: Omit<RenderOptions, "wrapper">
) => render(ui, { wrapper: AllProviders, ...options });

// Re-export everything from testing-library
export * from "@testing-library/react";

// Override render method
export { customRender as render };
