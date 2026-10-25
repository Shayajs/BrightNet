if (typeof bnFrame === "function") document.documentElement.classList.add("framed")

const frame = document.querySelector(".frame")
if (frame && document.documentElement.classList.contains("framed")) {
  frame.addEventListener("mousedown", (event) => {
    if (event.button !== 0 || event.target.closest("button")) return
    bnFrame("drag")
  })
  frame.addEventListener("dblclick", (event) => {
    if (event.target.closest("button")) return
    bnFrame("max")
  })
  document.querySelectorAll(".winbtns button").forEach((button) => {
    button.addEventListener("click", (event) => {
      event.stopPropagation()
      bnFrame(button.dataset.act)
    })
  })
  document.addEventListener("keydown", (event) => {
    if (event.key !== "F11") return
    event.preventDefault()
    event.stopPropagation()
    bnFrame("full")
  }, true)
}
