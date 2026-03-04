const renderMermaid = () => {
  if (!window.mermaid) {
    return;
  }

  window.mermaid.initialize({
    startOnLoad: false,
    securityLevel: "loose",
    theme: "neutral",
  });

  window.mermaid.run({ querySelector: ".mermaid" });
};

if (typeof document$ !== "undefined") {
  document$.subscribe(renderMermaid);
} else {
  window.addEventListener("load", renderMermaid);
}
