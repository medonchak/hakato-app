import React, { useEffect, useMemo, useState } from "react";

export default function NetworkCarousel({
  children,
  interval = 8000,
  transitionMs = 800,
}) {
  const items = useMemo(() => React.Children.toArray(children), [children]);
  const total = items.length;

  // ❗ старт з 1 (бо 0 — це клон останнього)
  const [index, setIndex] = useState(1);
  const [anim, setAnim] = useState(true);

  const slides = useMemo(() => {
    if (total === 0) return [];
    return [
      items[total - 1], // clone last
      ...items,
      items[0],         // clone first
    ];
  }, [items, total]);

  // autoplay — ЗАВЖДИ вперед
  useEffect(() => {
    if (total <= 1) return;

    const id = setInterval(() => {
      setIndex((i) => i + 1);
    }, interval);

    return () => clearInterval(id);
  }, [interval, total]);

  // 🧠 reset без анімації
  useEffect(() => {
    if (index === slides.length - 1) {
      // дійшли до clone first → стрибок на реальний 1
      setTimeout(() => {
        setAnim(false);
        setIndex(1);
      }, transitionMs);
    }

    if (index === 0) {
      // дійшли до clone last → стрибок на реальний last
      setTimeout(() => {
        setAnim(false);
        setIndex(slides.length - 2);
      }, transitionMs);
    }
  }, [index, slides.length, transitionMs]);

  // після “тихого” стрибка — знову вмикаємо анімацію
  useEffect(() => {
    if (!anim) {
      requestAnimationFrame(() => setAnim(true));
    }
  }, [anim]);

  if (total === 0) return null;

  const next = () => setIndex((i) => i + 1);
  const prev = () => setIndex((i) => i - 1);

  return (
    <div style={{ position: "relative", width: "100%" }}>
      {/* кнопки */}
      <button onClick={prev} style={{ ...arrow, left: 8 }}>‹</button>
      <button onClick={next} style={{ ...arrow, right: 8 }}>›</button>

      {/* viewport */}
      <div style={{ overflow: "hidden" }}>
        <div
          style={{
            display: "flex",
            width: `${slides.length * 100}%`,
            transform: `translateX(-${index * (100 / slides.length)}%)`,
            transition: anim ? `transform ${transitionMs}ms ease` : "none",
          }}
        >
          {slides.map((child, i) => (
            <div
              key={i}
              style={{
                width: `${100 / slides.length}%`,
                display: "flex",
                justifyContent: "center",
              }}
            >
              {child}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

/* кнопки */
const arrow = {
  position: "absolute",
  top: "50%",
  transform: "translateY(-50%)",
  width: 36,
  height: 36,
  borderRadius: 12,
  border: "none",

  background: "transparent", // ✅ реально прозорий
  color: "#ffffff",

  fontSize: 22,
  cursor: "pointer",
  zIndex: 10,
  opacity: 0.6,

  // ❌ НІЯКОГО backdropFilter
};
