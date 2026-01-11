import { getLocale, getTranslations } from "@/lib/i18n/server";
import { I18nProvider } from "@/lib/i18n/client";

interface I18nProviderWrapperProps {
  children: React.ReactNode;
}

export async function I18nProviderWrapper({ children }: I18nProviderWrapperProps) {
  const locale = await getLocale();
  const translations = await getTranslations(locale);

  return (
    <I18nProvider initialLocale={locale} initialTranslations={translations}>
      {children}
    </I18nProvider>
  );
}
