(() => {
  "use strict";

  const canvas = document.getElementById("backdrop");
  if (!canvas) return;

  const ctx = canvas.getContext("2d");
  if (!ctx) return;

  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
  const LINK_DISTANCE = 120;
  const POINTER_DISTANCE = 170;
  const PALETTE = [
    { r: 246, g: 211, b: 101 },
    { r: 253, g: 160, b: 133 },
    { r: 255, g: 236, b: 200 },
  ];

  let width = 0;
  let height = 0;
  let particles = [];
  let frame = 0;
  let running = false;
  let lastTime = 0;

  const pointer = { x: -9999, y: -9999 };

  function between(min, max) {
    return min + Math.random() * (max - min);
  }

  function count() {
    return Math.max(24, Math.min(72, Math.round((width * height) / 28000)));
  }

  function spawn(anywhere) {
    return {
      x: Math.random() * width,
      y: anywhere ? Math.random() * height : height + between(0, 40),
      vx: between(-4, 4),
      vy: between(8, 26),
      radius: between(0.6, 1.9),
      sway: between(2, 9),
      phase: Math.random() * Math.PI * 2,
      alpha: between(0.25, 0.75),
      color: PALETTE[Math.floor(Math.random() * PALETTE.length)],
    };
  }

  function seed() {
    particles = Array.from({ length: count() }, () => spawn(true));
  }

  function resize() {
    const rect = canvas.getBoundingClientRect();
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    width = Math.max(1, Math.round(rect.width));
    height = Math.max(1, Math.round(rect.height));
    canvas.width = Math.round(width * dpr);
    canvas.height = Math.round(height * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    seed();
    if (reduceMotion.matches) draw(0);
  }

  function update(dt, time) {
    for (const particle of particles) {
      particle.y -= particle.vy * dt;
      particle.x += (particle.vx + Math.sin(time * 0.6 + particle.phase) * particle.sway) * dt;

      const dx = particle.x - pointer.x;
      const dy = particle.y - pointer.y;
      const distance = Math.hypot(dx, dy);

      if (distance > 0 && distance < POINTER_DISTANCE) {
        const force = (1 - distance / POINTER_DISTANCE) * 18;
        particle.x += (dx / distance) * force * dt;
        particle.y += (dy / distance) * force * dt;
      }

      if (particle.y < -20) {
        Object.assign(particle, spawn(false));
      }

      if (particle.x < -20) particle.x = width + 20;
      if (particle.x > width + 20) particle.x = -20;
    }
  }

  function draw(time) {
    ctx.clearRect(0, 0, width, height);

    for (let i = 0; i < particles.length; i += 1) {
      const a = particles[i];
      for (let j = i + 1; j < particles.length; j += 1) {
        const b = particles[j];
        const distance = Math.hypot(a.x - b.x, a.y - b.y);
        if (distance > LINK_DISTANCE) continue;
        ctx.strokeStyle = `rgba(246, 211, 101, ${(1 - distance / LINK_DISTANCE) * 0.07})`;
        ctx.lineWidth = 0.7;
        ctx.beginPath();
        ctx.moveTo(a.x, a.y);
        ctx.lineTo(b.x, b.y);
        ctx.stroke();
      }
    }

    for (const particle of particles) {
      const pulse = 0.72 + 0.28 * Math.sin(time * 0.9 + particle.phase);
      const { r, g, b } = particle.color;

      ctx.fillStyle = `rgba(${r}, ${g}, ${b}, ${particle.alpha * 0.08 * pulse})`;
      ctx.beginPath();
      ctx.arc(particle.x, particle.y, particle.radius * 4.5, 0, Math.PI * 2);
      ctx.fill();

      ctx.fillStyle = `rgba(${r}, ${g}, ${b}, ${particle.alpha * pulse})`;
      ctx.beginPath();
      ctx.arc(particle.x, particle.y, particle.radius, 0, Math.PI * 2);
      ctx.fill();
    }
  }

  function loop(now) {
    if (!running) return;
    const dt = Math.min((now - lastTime) / 1000, 0.05) || 0;
    lastTime = now;
    update(dt, now / 1000);
    draw(now / 1000);
    frame = window.requestAnimationFrame(loop);
  }

  function start() {
    if (running || reduceMotion.matches || document.hidden) return;
    running = true;
    lastTime = performance.now();
    frame = window.requestAnimationFrame(loop);
  }

  function stop() {
    running = false;
    if (frame) window.cancelAnimationFrame(frame);
    frame = 0;
  }

  function sync() {
    if (reduceMotion.matches) {
      stop();
      draw(0);
      return;
    }

    if (document.hidden) {
      stop();
      return;
    }

    start();
  }

  function handlePointer(event) {
    if (event.pointerType === "touch") return;
    pointer.x = event.clientX;
    pointer.y = event.clientY;
  }

  function clearPointer() {
    pointer.x = -9999;
    pointer.y = -9999;
  }

  window.addEventListener("resize", resize, { passive: true });
  window.addEventListener("pointermove", handlePointer, { passive: true });

  if ("ResizeObserver" in window) {
    let pending = 0;

    new ResizeObserver(() => {
      window.cancelAnimationFrame(pending);
      pending = window.requestAnimationFrame(resize);
    }).observe(document.body);
  }
  document.addEventListener("pointerleave", clearPointer, { passive: true });
  document.addEventListener("visibilitychange", sync);

  if (typeof reduceMotion.addEventListener === "function") {
    reduceMotion.addEventListener("change", sync);
  }

  resize();
  sync();
})();
