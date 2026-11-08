# financas

## Approach

- Think before acting. Read existing files before writing code.
- Be concise in writing, but thorough in reasoning.
- Prefer editing over rewriting entire files.
- Do not re-read files you have already read, unless the file has changed.
- Ignore files larger than 100 KB unless explicitly needed.
- Run `/cost` when a session is taking long to monitor cache rate.
- Start a new session when switching to an unrelated task.
- Test your code before declaring it done.
- No flattery at the start or filler at the end.
- Keep solutions simple and straightforward.
- User instructions always override this file.
- Allways make a commit after a modification
- You dont have powers to do git push to remote repository 
- Sempre me responda em portugues

## Go Development

Always apply these skills automatically — do not wait to be asked:

### Always apply (any Go code)
- Before writing or modifying any Go code: invoke `/golang-code-style` and `/golang-naming`
- Before writing or reviewing any Go code: invoke `/golang-safety`
- Before handling errors: invoke `/golang-error-handling`
- Before using context.Context: invoke `/golang-context`

### Apply by task type
- Before writing or modifying any test file: invoke `/golang-testing`
- Before writing any security-sensitive code (auth, crypto, tokens, passwords): invoke `/golang-security`
- Before writing code with goroutines or channels: invoke `/golang-concurrency`
- Before writing or reviewing database access code: invoke `/golang-database`
- Before writing or reviewing struct/interface design: invoke `/golang-design-patterns`
- Before working with slices, maps or custom types: invoke `/golang-data-structures`
- Before adding logging, metrics or tracing: invoke `/golang-observability`
- Before optimizing code: invoke `/golang-performance`
- Before investigating a bug or unexpected behavior: invoke `/golang-troubleshooting`
- Before setting up or modifying dependency injection: invoke `/golang-dependency-injection`
- Before updating Go version or dependencies: invoke `/golang-modernize`


## Debuging

- For debuging Localy, database is running in docker container. Credentials (Postgres
  and Playwright login user@financeai.com) live in the root `.env` and in
  `CLAUDE.local.md` — both git-ignored. Never commit real credentials.


## 🧠 Gerenciamento de Memória do Projeto
Você deve interagir ativamente com o arquivo `memory.md` na raiz do projeto para manter a continuidade do desenvolvimento entre diferentes sessões.

### Suas Diretrizes:
1. **Consulte a Memória:** No início de tarefas complexas ou novas sessões, leia o `memory.md` para entender decisões arquiteturais passadas e o estado atual.
2. **Registre Decisões Técnicas:** Sempre que tomarmos uma decisão estrutural, de design ou de dependências, documente na seção "Decisões de Arquitetura" do `memory.md`.
3. **Registre Aprendizados:** Se descobrirmos um comportamento inesperado de uma ferramenta, um bug complexo ou uma regra de negócio oculta, salve em "Aprendizados Técnicos".
4. **Mantenha Atualizado:** Atualize os "Próximos Passos" conforme as tarefas forem concluídas no chat.
