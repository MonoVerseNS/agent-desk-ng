import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import { describe, it } from "node:test"

const source = await readFile(
  new URL("./conversation-switcher.tsx", import.meta.url),
  "utf8"
)

describe("support chat conversation switcher", () => {
  it("imports DropdownMenuGroup from dropdown-menu primitives", () => {
    assert.match(source, /DropdownMenuGroup/)
  })

  it("wraps DropdownMenuLabel inside DropdownMenuGroup to satisfy Base UI context (Error #31 prevention)", () => {
    assert.match(
      source,
      /<DropdownMenuGroup>[\s\S]*<DropdownMenuLabel[\s\S]*<\/DropdownMenuLabel>[\s\S]*<\/DropdownMenuGroup>/
    )
  })
})
