# Editor Integration

When creating or editing issues, ihj opens your editor with a Markdown file containing YAML frontmatter:

```yaml
---
# yaml-language-server: $schema=/automatically/generated/schema.json
type: Task
priority: Medium
status: Backlog
summary: "Your issue title here"
---
Description in Markdown goes here.
```

The `$schema` directive is generated automatically and points to a JSON Schema for the current workspace. This enables autocompletion for field names and enum values in editors with yaml-language-server support.

If you use a vim-like editor, ihj automatically:

- Positions the cursor on the summary field (or description body)
- Enters insert mode

Save and quit to submit. If validation fails or the API rejects the request, you'll be offered the choice to re-edit, copy to clipboard, or abort.

## otter.nvim setup

To get YAML autocompletion in Neovim, add this to your config. It detects ihj's frontmatter schema directive and activates otter.nvim automatically:

```lua
local function activate_ihj_otter()
  if vim.bo.filetype ~= "markdown" then
    return
  end

  local lines = vim.api.nvim_buf_get_lines(0, 0, 2, false)
  if #lines < 2 then
    return
  end

  local is_frontmatter = lines[1]:match("^%-%-%-%s*$")
  local has_schema = lines[2]:match("^#%s*yaml%-language%-server:%s*$schema=")

  if is_frontmatter and has_schema then
    local ok, otter = pcall(require, "otter")
    if ok then
      otter.activate({ "yaml" })
    else
      vim.notify("otter.nvim not found", vim.log.levels.WARN)
    end
  end
end

vim.api.nvim_create_autocmd({ "BufReadPost", "BufEnter" }, {
  pattern = "*.md",
  callback = activate_ihj_otter,
})
```
