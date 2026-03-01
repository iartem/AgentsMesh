"use client";

import { useTranslations } from "next-intl";

/**
 * DemoVideo - Product demo video section with YouTube embed
 *
 * Displays the AgentsMesh product demo video between HeroSection and AgentLogos.
 * Uses youtube-nocookie.com for privacy-enhanced mode with lazy loading.
 */
export function DemoVideo() {
  const t = useTranslations();

  return (
    <section className="py-16 sm:py-20 relative overflow-hidden">
      {/* Background gradient */}
      <div className="absolute inset-0 bg-gradient-to-b from-transparent via-primary/5 to-transparent" />

      <div className="container mx-auto px-4 sm:px-6 lg:px-8 relative z-10">
        <div className="text-center max-w-3xl mx-auto mb-10">
          <h2 className="text-3xl sm:text-4xl font-bold mb-4">
            {t("landing.demoVideo.title")}{" "}
            <span className="text-primary">{t("landing.demoVideo.titleHighlight")}</span>
          </h2>
        </div>

        {/* Video container */}
        <div className="max-w-4xl mx-auto relative group">
          {/* Glow effect */}
          <div className="absolute -inset-1 bg-gradient-to-r from-primary/30 to-secondary/30 rounded-2xl blur-2xl opacity-30 group-hover:opacity-50 transition-opacity duration-500" />
          
          <div className="relative rounded-xl overflow-hidden border border-border/50 shadow-2xl shadow-primary/10 bg-black ring-1 ring-white/10" style={{ aspectRatio: "16/9" }}>
            <iframe
              src="https://www.youtube-nocookie.com/embed/FZrUO0tim0U?rel=0&modestbranding=1"
              title={t("landing.demoVideo.iframeTitle")}
              allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
              allowFullScreen
              loading="lazy"
              className="absolute inset-0 w-full h-full"
            />
          </div>

          {/* Highlight tags */}
          <div className="flex flex-wrap justify-center gap-3 mt-8">
            {[
              t("landing.demoVideo.highlight1"),
              t("landing.demoVideo.highlight2"),
              t("landing.demoVideo.highlight3"),
            ].map((label) => (
              <span
                key={label}
                className="inline-flex items-center gap-2 px-4 py-2 rounded-full text-sm font-medium bg-secondary/50 text-foreground border border-border/50 backdrop-blur-sm shadow-sm transition-all hover:bg-primary/10 hover:border-primary/20 hover:text-primary hover:-translate-y-0.5 cursor-default"
              >
                <svg className="w-4 h-4 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
                {label}
              </span>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
