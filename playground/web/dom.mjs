// DOM helpers of the playground page.

const SVG_NAMESPACE = "http://www.w3.org/2000/svg";
const LONG_PRESS_MS = 500;
/** A pointer that moves more than this many pixels is not a long press. */
const LONG_PRESS_SLOP = 10;
/** The time after a long press in which a click or contextmenu event belongs to it. */
const LONG_PRESS_EVENT_MS = 800;

/**
 * element returns a new HTML element. Each field of properties but attributes is set as a property,
 * each field of properties.attributes as an attribute, and the children are appended.
 */
export function element(tag, properties = {}, ...children) {
  const { attributes = {}, ...fields } = properties;
  const node = Object.assign(document.createElement(tag), fields);
  for (const [name, value] of Object.entries(attributes)) node.setAttribute(name, value);
  node.append(...children);
  return node;
}

/** icon returns an svg element that shows the symbol #id of the page. */
export function icon(id, className = "ex-icon") {
  const svg = document.createElementNS(SVG_NAMESPACE, "svg");
  svg.setAttribute("class", className);
  svg.setAttribute("aria-hidden", "true");
  const use = document.createElementNS(SVG_NAMESPACE, "use");
  use.setAttribute("href", "#" + id);
  svg.append(use);
  return svg;
}

/**
 * longPress calls onPress(x, y, event) when a touch or pen holds still on target for 500 ms. The
 * click and contextmenu events right after a long press do nothing.
 */
export function longPress(target, onPress) {
  let timer = 0;
  let startX = 0;
  let startY = 0;
  let pressedAt = 0;
  const cancel = () => clearTimeout(timer);
  target.addEventListener("pointerdown", event => {
    if (event.pointerType === "mouse") return;
    cancel();
    startX = event.clientX;
    startY = event.clientY;
    timer = setTimeout(() => {
      pressedAt = Date.now();
      onPress(startX, startY, event);
    }, LONG_PRESS_MS);
  });
  target.addEventListener("pointermove", event => {
    if (Math.hypot(event.clientX - startX, event.clientY - startY) > LONG_PRESS_SLOP) cancel();
  });
  for (const type of ["pointerup", "pointercancel", "pointerleave"]) target.addEventListener(type, cancel);
  const swallowAfterPress = event => {
    if (Date.now() - pressedAt >= LONG_PRESS_EVENT_MS) return;
    event.preventDefault();
    event.stopImmediatePropagation();
  };
  target.addEventListener("contextmenu", swallowAfterPress, true);
  target.addEventListener("click", swallowAfterPress, true);
}
