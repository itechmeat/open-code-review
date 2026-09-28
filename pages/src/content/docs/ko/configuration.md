---
title: 설정
sidebar:
  order: 5
---

설정 파일은 `~/.opencodereview/config.json`에 있습니다. 편집하는 방법은 세 가지입니다:

- **대화형 TUI** — `ocr config provider` / `ocr config model`. 메뉴가 안내합니다.
- **커맨드라인** — `ocr config set <key> <value>`. 스크립트와 CI에 적합합니다.
- **직접 편집(권장하지 않음)** — JSON 파일을 직접 수정합니다(다음 `ocr config set` 기록 때 다시 포맷됩니다).

## 모델 설정 {#configuring-a-model}

### 권장: 대화형 설정 {#recommended-interactive-setup}

```bash
ocr config provider
```

내장 프로바이더나 커스텀 프로바이더를 고르고 API 키를 입력한 뒤 모델을 선택하면, 명령이 모든 값을 설정 파일에 저장하고 `ocr llm test`를 한 번 실행해 엔드포인트를 검증합니다. 나중에 모델을 바꾸려면:

```bash
ocr config model
```

### 비대화형 설정 (CI / TUI가 없는 환경) {#non-interactive-setup-ci-no-tui-environments}

`ocr config set`으로 같은 설정 파일에 기록합니다:

```bash
ocr config set provider                    anthropic
ocr config set model                       claude-opus-4-6
ocr config set providers.anthropic.api_key sk-ant-xxxxxxxxxx
```

### 내장 프로바이더 {#built-in-providers}

다음 프로바이더는 Base URL과 프로토콜이 미리 설정된 채 OCR에 내장되어 있습니다. 선택한 뒤 API 키만 채우면 됩니다. `providers.<name>.api_key`가 비어 있으면 OCR은 해당 환경 변수로 대체합니다.

| 이름 | 프로토콜 | Base URL | API 키 환경 변수 |
|---|---|---|---|
| `claude-code` | claude-code | — (로컬 `claude` CLI 실행) | — (Claude Code 로그인) |
| `anthropic` | anthropic | `https://api.anthropic.com` | `ANTHROPIC_API_KEY` |
| `bedrock` | anthropic-bedrock | `aws_region`에서 결정 | — (AWS 자격 증명 체인) |
| `openai` | openai | `https://api.openai.com/v1` | `OPENAI_API_KEY` |
| `openai-responses` | openai-responses | `https://api.openai.com/v1` | `OPENAI_RESPONSES_API_KEY` |
| `openrouter` | openai | `https://openrouter.ai/api/v1` | `OPENROUTER_API_KEY` |
| `gemini` | openai | `https://generativelanguage.googleapis.com/v1beta/openai` | `GEMINI_API_KEY` |
| `dashscope` | openai | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `DASHSCOPE_API_KEY` |
| `dashscope-tokenplan` | openai | `https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1` | `DASHSCOPE_TOKENPLAN_KEY` |
| `volcengine` | openai | `https://ark.cn-beijing.volces.com/api/v3` | `ARK_API_KEY` |
| `deepseek` | openai | `https://api.deepseek.com` | `DEEPSEEK_API_KEY` |
| `tencent-tokenhub` | openai | `https://tokenhub.tencentmaas.com/v1` | `TENCENT_TOKENHUB_API_KEY` |
| `hy-tokenplan` | openai | `https://api.lkeap.cloud.tencent.com/plan/v3` | `TENCENT_HUNYUAN_TOKENPLAN_KEY` |
| `iflytek` | openai | `https://spark-api-open.xf-yun.com/v1` | `SPARK_API_KEY` |
| `kimi` | openai | `https://api.moonshot.cn/v1` | `MOONSHOT_API_KEY` |
| `kimi-global` | openai | `https://api.moonshot.ai/v1` | `MOONSHOT_GLOBAL_API_KEY` |
| `z-ai` | openai | `https://open.bigmodel.cn/api/paas/v4` | `Z_AI_API_KEY` |
| `mimo` | openai | `https://api.xiaomimimo.com/v1` | `MIMO_API_KEY` |
| `minimax` | openai | `https://api.minimax.io/v1` | `MINIMAX_GLOBAL_API_KEY` |
| `minimax-cn` | openai | `https://api.minimaxi.com/v1` | `MINIMAX_API_KEY` |
| `baidu-qianfan` | openai | `https://qianfan.baidubce.com/v2` | `QIANFAN_API_KEY` |
| `siliconflow`  | openai | `https://api.siliconflow.com/v1` | `SILICONFLOW_GLOBAL_API_KEY` |
| `siliconflow-cn`  | openai | `https://api.siliconflow.cn/v1` | `SILICONFLOW_API_KEY` |
| `novita` | openai | `https://api.novita.ai/openai` | `NOVITA_API_KEY` |
| `xai` | openai | `https://api.x.ai/v1` | `XAI_API_KEY` |

내장 프로바이더의 모델 목록은 `ocr config model`에서 선택할 때 제안하는 목록이며
`--model`을 제한하지 않습니다. 지정한 모델이 내장 목록과
`providers.<name>.models` 모두에 없으면 OCR은 stderr에 경고를 출력합니다.
모델의 유효성은 요청을 보낼 때 프로바이더가 확인합니다. 사용자 정의 프로바이더에는
기존 `--model` 검증 규칙이 적용됩니다.

### 내장 프로바이더의 Base URL 재정의 {#overriding-a-built-in-provider-s-base-url}

모든 내장 프로바이더에는 미리 설정된 Base URL이 있습니다(위 표 참고). 내장 프로바이더를 다른 엔드포인트로 보내려면 `providers.<name>.url`을 설정합니다(예: 자체 호스팅 LiteLLM 게이트웨이는 미리 설정된 기본값 `http://localhost:4000/v1`에 있는 경우가 드뭅니다):

```bash
ocr config set provider                   litellm
ocr config set model                      openai/gpt-5.4
ocr config set providers.litellm.api_key  "$LITELLM_API_KEY"
ocr config set providers.litellm.url      https://gateway.internal:8000/v1
```

설정한 `url`이 미리 설정된 Base URL보다 우선합니다. `providers.<name>.url`이 비어 있으면(또는 지우면) OCR은 기본값으로 돌아가므로, 엔드포인트가 다를 때만 설정하면 됩니다.

### AWS Bedrock {#aws-bedrock}

`bedrock`은 `anthropic`과 같은 Messages API를 사용하지만, 요청에 API 키를 싣는 대신 표준 AWS 자격 증명 체인으로 SigV4 서명을 하며 호스트는 리전이 결정합니다. 설정할 `api_key`가 없고, 서명을 대신할 키도 받지 않습니다:

```bash
ocr config set provider                      bedrock
ocr config set model                         us.anthropic.claude-sonnet-4-6
ocr config set providers.bedrock.aws_region  us-west-2
ocr config set providers.bedrock.aws_profile example-profile
```

| 필드 | 의미 |
|---|---|
| `providers.bedrock.aws_region` | 요청을 처리할 `bedrock-runtime` 호스트의 리전. 비어 있으면 `AWS_REGION`이나 활성 프로필로 대체합니다. |
| `providers.bedrock.aws_profile` | 자격 증명을 가져올 명명 프로필. 비어 있으면 `AWS_PROFILE`이나 환경의 자격 증명 체인으로 대체합니다. |

두 필드 모두 선택 사항이며, 비워 두면 다른 AWS 도구와 마찬가지로 표준 체인이 결정합니다. 값을 고정해 두면 `AWS_PROFILE`을 먼저 내보내지 않아도 실행을 재현할 수 있는데, 기본 프로필이 다른 CI 러너에서 특히 유용합니다.

모델 식별자는 계정 **과** 리전 단위로 정해지므로 OCR이 제공하는 목록은 닫힌 집합이 아니라 출발점입니다. 목록에 없더라도 계정에서 유효한 추론 프로필 ID나 애플리케이션 추론 프로필 ARN이면 그대로 받습니다. 계정에서 무엇을 쓸 수 있는지는 `aws bedrock list-inference-profiles --region <region>`으로 확인하세요. 최신 계열에서는 `-v1:0` 같은 버전 접미사가 유효하지 않습니다.

bedrock에는 설정된 URL이 없고 호스트를 리전이 결정하므로, `ocr llm test`는 URL 대신 리전과 프로필을 표시합니다:

```
Source: provider:bedrock
Region: us-east-1
Profile: example-profile
Model:  claude-sonnet-5
✓ Connection test successful
```

Bedrock은 `llm.protocol`이나 `OCR_LLM_PROTOCOL`로는 사용할 수 **없습니다**. 이 블록은 URL 하나와 토큰 하나를 기술하는 구조라 리전이나 프로필을 담을 자리가 없고, bedrock은 이 블록이 담는 두 값 중 어느 것도 쓰지 않습니다. 그래서 이 조합은 받아들인 뒤 무시하는 대신 거부합니다.

### Claude Code (구독) {#claude-code-subscription}

`claude-code`는 머신에 설치된 Claude Code CLI로 모든 요청을 처리합니다. OCR은 사용자가 직접 설치한, 수정되지 않은 `claude -p`를 실행하므로 리뷰 비용은 `claude`가 로그인한 계정, 보통 Claude 구독에 청구됩니다. URL, API 키, 설정 항목을 지정할 필요가 없으며 OCR은 Claude 자격 증명을 읽지 않습니다.

이 프로바이더는 내장 기본값이기도 합니다. 다른 어떤 소스도 엔드포인트를 설정하지 않았고([프로바이더 해석 순서](#provider-resolution-order) 참고) `claude` 실행 파일을 찾을 수 있으면, OCR은 `claude-code`와 `opus` 모델로 리뷰합니다.

```bash
ocr review --from main --to HEAD                    # 설정 파일 불필요
ocr review --provider claude-code --model sonnet    # 이번 실행만 다른 모델
ocr config set provider claude-code                 # 설정 파일에 고정
```

모델은 별칭(`opus`, `sonnet`, `haiku`, `fable`) 또는 전체 모델 ID입니다. 이 프로바이더는 어떤 `--model` 값이든 받으며, 실제로 쓸 수 있는지는 `claude` 로그인이 결정합니다.

| 변수 | 의미 |
|---|---|
| `OCR_CLAUDE_CODE_BIN` | `claude` 실행 파일 경로. 기본값은 `PATH`의 `claude`입니다. |
| `OCR_CLAUDE_CODE_EFFORT` | Claude Code에 전달하는 추론 강도: `low`(기본값), `medium`, `high`, `xhigh`, `max`, 또는 Claude Code에 맡기는 `auto`. |

`claude` 프로세스는 `ANTHROPIC_LOG`를 제외한 모든 `ANTHROPIC_*` 변수, `CLAUDE_CODE_USE_*` 백엔드 스위치(Bedrock, Vertex, Foundry, 게이트웨이), `CLAUDE_CODE_API_*` 엔드포인트와 키 설정, `CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST`를 제거한 환경에서 시작합니다. 그래서 셸에 export된 키나 게이트웨이 때문에 리뷰 과금이 구독에서 API로 조용히 바뀌는 일은 없습니다. Anthropic 약관은 수정되지 않은 Claude Code 바이너리를 통해서만 구독 사용을 허용합니다. 자세한 내용은 [Legal and compliance](https://code.claude.com/docs/en/legal-and-compliance)를 참고하세요.

`ocr llm test`는 URL 대신 실행 파일을 표시합니다:

```
Source: claude CLI on PATH
CLI:    /usr/local/bin/claude
Model:  opus
✓ Connection test successful
```

`OCR_CLAUDE_CODE_BIN`이 실행 파일을 정했다면 `Source:`는 `OCR_CLAUDE_CODE_BIN`이고, 프로바이더를 `--provider`나 설정 파일로 골랐다면 `provider:claude-code`입니다.

Claude Code가 구독 사용량 한도를 보고하면 `ocr review`는 남은 그룹의 디스패치를 멈추고, 이미 나온 결과를 유지하며, `[ocr] Run stopped early: ...`를 출력한 뒤 `ocr review --resume <id>` 안내와 함께 `3`으로 종료합니다(아직 아무것도 리뷰하지 못했다면 `1`로 종료). 한도 기간이 초기화된 뒤 재개하거나 `--concurrency`를 낮추세요. `ocr scan`은 아직 이 메시지를 출력하지 않습니다. `claude`가 로그인되어 있지 않거나 이 프로바이더에 비해 너무 오래된 경우에도 실행이 같은 방식으로 멈춥니다. `claude`를 실행해 `/login`하거나 Claude Code를 업데이트한 뒤 재개하세요.

### 프로바이더 해석 순서 {#provider-resolution-order}

`--provider`가 없으면 OCR은 완전한 엔드포인트를 주는 첫 번째 소스를 사용합니다:

1. 설정 파일 `~/.opencodereview/config.json`(`provider` 또는 레거시 `llm` 블록).
2. `OCR_LLM_URL` / `OCR_LLM_TOKEN` / `OCR_LLM_MODEL`.
3. `ANTHROPIC_BASE_URL` / `ANTHROPIC_AUTH_TOKEN` / `ANTHROPIC_MODEL`.
4. 셸 rc 파일에 있는 같은 export.
5. `PATH`의 Claude Code CLI(또는 `OCR_CLAUDE_CODE_BIN`): 프로바이더 `claude-code`, 모델 `opus`.

설정 파일이 항상 우선합니다. 설정 파일이 프로바이더를 지정했다면 설정이 잘못됐더라도 그 프로바이더의 오류가 그대로 보고되고, 반쯤 채운 `llm` 블록도 오류로 보고됩니다. 어느 쪽도 Claude Code로 대체되지 않습니다. 프로바이더가 설정되지 않았으면 `ocr config get provider`는 `claude-code`를 출력하고 그 출처를 stderr에 설명합니다.

`ocr review`, `ocr scan`, `ocr llm test`의 `--provider <preset>`은 설정을 바꾸지 않고 이번 실행의 프로바이더를 고릅니다. 내장 프리셋은 API 키가 필요 없거나(`claude-code`, `bedrock`) 그 API 키 환경 변수가 설정돼 있으면 `providers.<name>` 항목 없이도, 설정 파일 없이도 동작합니다. 모델은 `--model`, 항목의 `model`, 최상위 `model`(`--provider`가 설정된 프로바이더를 가리킬 때만 유지), 프리셋 기본값 순으로 정해집니다. 기본값이 있는 프리셋은 `claude-code`뿐이므로 `bedrock`은 여전히 `--model`이 필요합니다. `--model`은 프리셋 모델 목록, 항목의 `models`, 그리고 항목이나 최상위 `model`에 설정된 모델을 받습니다. 프리셋 목록에 아직 없는 모델이어도 됩니다.

```bash
ocr review --provider z-ai-coding --model <id>    # 이번 실행만 다른 프로바이더
ocr config set provider z-ai-coding               # 기본값으로 지정
```

### 커스텀 프로바이더 {#custom-providers}

위 표에 없는 프로바이더 이름은 커스텀으로 취급하며 최소한 `url`과 `protocol`을 지정해야 합니다(`protocol`은 `anthropic`, `openai`, `openai-responses`, `anthropic-bedrock` 중 하나):

```bash
ocr config set provider                             my-gateway
ocr config set custom_providers.my-gateway.url      https://gateway.internal.com/v1
ocr config set custom_providers.my-gateway.protocol openai
ocr config set custom_providers.my-gateway.model    llama-3-70b
ocr config set custom_providers.my-gateway.api_key  "$MY_API_KEY"
```

프로바이더나 모델이 OpenAI Responses API(`/v1/responses`)를 요구하면 `openai-responses`를 사용합니다:

```bash
ocr config set provider                                               openai-responses-gateway
ocr config set custom_providers.openai-responses-gateway.url          https://api.openai.com/v1
ocr config set custom_providers.openai-responses-gateway.protocol     openai-responses
ocr config set custom_providers.openai-responses-gateway.model        gpt-5
ocr config set custom_providers.openai-responses-gateway.api_key      "$OPENAI_API_KEY"
```

`anthropic-bedrock` 프로토콜을 쓰는 커스텀 프로바이더는 호스트를 리전이 결정하므로 `url`이 필요 없고, 내장 프로바이더와 같은 AWS 필드를 받습니다. 두 번째 리전이나 프로필에 별도 항목을 주려면 이렇게 합니다:

```bash
ocr config set provider                                bedrock-eu
ocr config set custom_providers.bedrock-eu.protocol    anthropic-bedrock
ocr config set custom_providers.bedrock-eu.aws_region  eu-west-1
ocr config set custom_providers.bedrock-eu.aws_profile eu-profile
ocr config set custom_providers.bedrock-eu.model       eu.anthropic.claude-sonnet-4-6
```

`url`은 API Base URL이든 전체 `/responses` 엔드포인트든 상관없습니다. OCR이 어느 쪽이든 정규화합니다.

Ollama로 서빙하는 로컬 모델도 로컬 OpenAI 호환 엔드포인트를 가리키는 커스텀 프로바이더일 뿐입니다:

```bash
ocr config set provider                          ollama
ocr config set custom_providers.ollama.url       http://127.0.0.1:11434/v1
ocr config set custom_providers.ollama.protocol  openai
ocr config set custom_providers.ollama.model     qwen3:32b
ocr config set custom_providers.ollama.api_key   ollama
```

Ollama는 API 키를 무시하지만 커스텀 프로바이더에는 비어 있지 않은 `api_key`가 필요하므로(커스텀 프로바이더에는 환경 변수 대체가 없음) 아무 자리 표시자 값이나 설정하세요. 모델 자체는 네이티브 도구 호출을 지원해야 합니다. 모델을 고르기 전에 FAQ의 ["No tool calls parsed" (local models / Ollama)](../faq/#no-tool-calls-parsed-local-models-ollama)를 참고하세요.

### 타임아웃 {#timeouts}

태스크 타임아웃과 개별 요청 타임아웃은 서로 독립적입니다:

- `ocr review --timeout`과 `ocr scan --timeout`은 동시 실행 태스크의 시간 예산을
  **분** 단위로 설정합니다(기본값 **15**, `0`이면 태스크 기한을 비활성화).
  이 예산에는 태스크의 모든 LLM 호출, 도구 실행, 재시도 대기가 포함됩니다.
  review에서는 effort의 리뷰 라운드 수만큼 예산을 늘립니다.
- 개별 HTTP 요청 타임아웃은 **초** 단위입니다(기본값 **300**).
  `--timeout`을 늘려도 요청 타임아웃은 바뀌지 않으며, 태스크 기한을 비활성화해도
  요청 타임아웃은 비활성화되지 않습니다. 태스크의 남은 예산이 더 짧으면 그 예산이 우선합니다.

LLM 요청마다 HTTP 타임아웃이 있으며 기본값은 **300초**입니다. 느린 로컬 모델(또는 큰 파일)에는 더 필요할 수 있습니다. 범위가 좁은 것부터 세 가지 설정이 있습니다:

- `providers.<name>.timeout_sec` / `custom_providers.<name>.timeout_sec` — 프로바이더별, 초 단위.
- `llm.timeout_sec` — 레거시 `llm` 섹션용, 초 단위.
- `OCR_LLM_TIMEOUT` 환경 변수 — 정수 초. 모든 해석 경로에서 설정 파일 값보다 우선합니다.

두 `timeout_sec` 키 모두 `ocr config set`으로 설정할 수 있습니다:

```json
{
  "custom_providers": {
    "ollama": { "url": "http://127.0.0.1:11434/v1", "protocol": "openai", "timeout_sec": 900 }
  }
}
```

예를 들어 `OCR_LLM_TIMEOUT=900 ocr review --timeout 30`은 태스크 예산 내에서
개별 요청을 최대 15분까지 허용합니다. 이 환경 변수나 `timeout_sec`를 설정하지 않으면
`--timeout 30`을 지정해도 요청은 5분 후에 타임아웃됩니다.

개별 요청 기한이 만료되면 요청을 자동으로 다시 보내지 않고 호출을 종료합니다.
응답 본문이나 스트림을 읽는 도중 타임아웃된 경우도 마찬가지입니다. 지속적으로 느린
요청은 같은 제한 시간으로 재시도하기보다 요청 타임아웃을 늘려야 합니다.
기존 SDK 재시도 정책은 바뀌지 않습니다. 재시도 가능한 연결 및 HTTP 오류는 백오프와
프로바이더의 `Retry-After` 지시에 따라 최대 5회 재시도합니다. SDK의 각 시도 기한은
재시도 대기 시간에도 적용됩니다. 취소되거나 태스크 기한이 만료되면 요청과 재시도 대기가
모두 중단됩니다.

진단 메시지는 `LLM request timeout`(`OCR_LLM_TIMEOUT` 또는 프로바이더의 `timeout_sec` 확인)과
`caller deadline exceeded`를 구분합니다. 후자는 호출한 작업의 컨텍스트 기한이 만료되었다는
뜻입니다. review/scan 태스크 기한이 만료된 경우에는 `--timeout`을 확인하세요.
백그라운드 메모리 압축이나 `ocr llm test` 같은 다른 작업에는 각각 독립적인 기한이 있습니다.

### 명령으로 API 키 가져오기 {#api-key-from-a-command}

키를 설정 파일에 저장하는 대신 `api_key_cmd`가 실행 시점에 비밀 관리자(1Password, `pass`, `gopass` 등)에서 가져옵니다. 앞뒤 공백을 제거한 한 줄짜리 stdout이 키가 됩니다. 레거시 `llm` 블록에서는 같은 옵션을 `auth_token_cmd`로 사용할 수 있습니다.

```bash
ocr config set providers.anthropic.api_key_cmd "op read op://dev/anthropic/api-key"
```

OS 키링도 이미 설치된 도구를 통해 같은 방식으로 동작하므로, 키가 `config.json`이 아니라 Keychain이나 Secret Service에 남습니다:

```bash
# macOS Keychain
ocr config set providers.anthropic.api_key_cmd \
  "security find-generic-password -s ocr-anthropic -w"

# Linux (Secret Service: GNOME Keyring, KWallet 등)
ocr config set providers.anthropic.api_key_cmd \
  "secret-tool lookup service ocr-anthropic"
```

우선순위: 정적 `api_key`가 항상 이깁니다(둘 다 설정하면 명령은 무시되고 경고가 출력됩니다). 그다음 `api_key_cmd`가 실행되며, 둘 다 없을 때만 OCR이 프로바이더의 환경 변수로 대체합니다.

명령은 `ocr` 실행마다 한 번 돌고 반드시 성공해야 합니다. 0이 아닌 종료 코드, 빈 출력, 여러 줄 출력, 64KiB 초과 출력은 하드 에러입니다(OCR은 조용히 대체하지 않습니다). 프롬프트에 응답하는 시간을 포함해 60초 안에 끝나야 합니다. 명령은 터미널의 stdin과 stderr를 물려받으므로 대화형 프롬프트(pinentry, Touch ID)가 표시되고 응답할 수 있습니다. 명령이 stdout 파이프를 잡고 있는 백그라운드 데몬(`gpg-agent`, 최초 실행 시의 `op` 데몬)을 남기면 자격 증명은 도착하지만 `ocr` 실행마다 그 파이프가 닫히길 기다리며 5초씩 멈춥니다. 데몬의 출력을 리다이렉트(`>/dev/null 2>&1`)하면 대기가 사라집니다.

Windows에서는 명령이 `sh`가 아니라 `cmd.exe`로 실행되므로 한쪽용으로 작성한 명령이 다른 쪽에서 그대로 돌지 않는 경우가 많습니다. `%VAR%`와 `^`는 `cmd.exe` 메타 문자이고, `$VAR` 확장과 `\` 이스케이프는 거기서 적용되지 않습니다. 따옴표로 감싼 인자는 그대로 전달되므로 `op read "op://Private/My Vault/api-key"`는 그대로 동작합니다.

값이 셸 명령으로 실행되므로 `config.json`은 신뢰된 입력입니다. 소유자를 본인으로 유지하고 다른 사용자가 쓸 수 없게 하세요(OCR은 `0600` 권한으로 기록합니다).

### 추가 재시도 상태 코드 {#additional-retry-status-codes}

일부 LLM 프로바이더는 일시적 오류에 비표준 4xx 상태 코드를 사용합니다. 예를 들어 레이트 리밋에 `403`이나 `400`을 반환하기도 합니다. `retry_codes`를 설정하면 OCR이 기존 SDK 재시도 메커니즘으로 이런 요청을 재시도합니다.

`retry_codes`는 정수 배열이며 `llm.retry_codes` 또는 `custom_providers.<name>.retry_codes`로 설정할 수 있습니다. `ocr config set`을 쓸 때는 코드를 쉼표로 구분해 전달합니다:

```bash
ocr config set llm.retry_codes 403,400
ocr config set custom_providers.my-gateway.retry_codes 403,400
```

4xx HTTP 상태 코드만 허용됩니다. `408`, `409`, `429`는 SDK가 이미 재시도합니다. 설정 파일에서 읽을 때 이런 중복 코드는 무시되며, `ocr config set`으로 전달하면 OCR이 경고를 출력하고 저장 값에서 제외합니다. 5xx 응답은 모두 SDK가 이미 재시도하므로 `retry_codes`에 추가할 수 없습니다.

### 프롬프트 상한 {#prompt-limit}

`max_tokens`는 서브태스크 하나(파일 하나 또는 관련된 파일 묶음)에 대한 **프롬프트**(입력) 상한입니다. 내장 템플릿의 기본값은 `ocr review` 200,000토큰, `ocr scan` 58,888토큰입니다. 컨텍스트 윈도가 다른 모델에서는 `max_tokens`를 저장해 바꿉니다:

```bash
ocr config set max_tokens 400000
```

이 설정은 `ocr review`와 `ocr scan` 모두에 적용됩니다. 저장된 설정을 바꾸지 않고 일회성으로 재정의하려면 `--max-tokens`를 사용합니다:

```bash
ocr review --max-tokens 400000
ocr scan --max-tokens 400000
```

실행별 플래그가 `max_tokens`보다 우선하고, 둘 다 없으면 OCR은 내장 작업 템플릿 기본값을 사용합니다. 이 상한은 모델의 **출력** 상한(`MAX_COMPLETION_TOKENS`, 두 템플릿 모두 `16384`)이나 실행 전체 토큰 사용량을 제한하는 `--max-tokens-budget`과는 별개입니다. `ocr config unset max_tokens`로 내장 기본값을 복원합니다.

### 리뷰 강도 (effort) {#review-effort}

`effort`는 서브태스크마다 리뷰를 몇 라운드 돌릴지 정합니다. `low` = 1라운드, `medium`(기본값) = 2라운드, `high` = 3라운드입니다. 라운드가 늘어나면 더 많은 문제를 찾지만 비용도 그만큼 늘어납니다.

```bash
ocr config set effort high
ocr config unset effort      # 기본값 medium으로 복귀
```

`--effort low|medium|high`는 한 번의 실행에 한해 저장된 값을 재정의합니다.

### 연결 검증 {#verify-connectivity}

```bash
ocr llm test
```

### 기존 환경 변수 재사용 {#reuse-existing-environment-variables}

Claude Code의 `ANTHROPIC_*` 또는 OCR 자체의 `OCR_LLM_*` 환경 변수를 이미 설정해 두었다면 OCR이 자동으로 인식합니다. 설정 파일이 필요 없습니다. 설정 파일 및 Claude Code 기본값과의 우선순위는 [프로바이더 해석 순서](#provider-resolution-order)를 참고하세요.

### CC-Switch 사용 {#using-cc-switch}

[CC-Switch](https://github.com/farion1231/cc-switch)를 [라우팅 서비스](https://www.ccswitch.io/en/docs?section=proxy&item=service)와 함께 사용 중이라면, 프로바이더 `url`을 로컬 프록시로 향하게 하면 됩니다. 다른 설정은 필요 없습니다:

```bash
# Claude (Anthropic 호환)
ocr config set providers.anthropic.url http://127.0.0.1:15721

# Codex / OpenAI 호환 — 해당 프로바이더의 url 키를 설정
ocr config set providers.<name>.url http://127.0.0.1:15721/v1
```

`api_key`는 아무 값이어도 됩니다. `extra_body`(및 다른 프로바이더별 필드)는 평소처럼 적용됩니다.

### 벤더 고유 필드 전송 {#send-vendor-specific-fields}

일부 프로바이더는 비표준 요청 필드를 요구합니다(Bedrock 스타일의 `thinking` 등). 소스를 고치지 않고 보내려면 모든 요청에 병합되는 `extra_body`를 사용합니다:

```bash
ocr config set providers.anthropic.extra_body '{"thinking":{"type":"disabled"}}'
```

### 프롬프트 캐싱을 위한 세션 어피니티 {#session-affinity-for-prompt-caching}

OCR은 LLM 대화마다 리뷰 세션과 그 안의 작업 범위로 한정된 프롬프트 캐시 어피니티 키(`<session-id>-<task-type>-<scope-hash>`)를 만듭니다. 프롬프트 캐시는 접두사로 매칭되므로, 대화별 키는 실행 전체를 핫 키 하나에 고정하는 대신 늘어나는 각 대화(파일 하나의 리뷰 도구 루프 등)를 일관된 캐시 노드에 붙잡아 둡니다. 세션 ID 접두사 덕분에 프로바이더 쪽 캐시 로그를 `ocr session` 기록과 대조할 수 있습니다.

사용하려면 프로바이더가 키를 기대하는 자리의 `extra_headers`나 `extra_body` 값에 `{ocr_session_key}` 템플릿 변수를 넣습니다. OCR이 요청마다 해당 대화의 키로 치환하며, 넣지 않으면 아무것도 보내지 않습니다:

```bash
# OpenAI 스타일 요청 본문 필드로 전달(예: prompt_cache_key)
ocr config set providers.openai.extra_body '{"prompt_cache_key": "{ocr_session_key}"}'

# HTTP 헤더로 전달(예: x-session-affinity)
ocr config set custom_providers.my-gateway.extra_headers "x-session-affinity={ocr_session_key}"
```

## 리뷰 언어 설정 {#configuring-the-review-language}

`language`는 리뷰 코멘트를 어떤 언어로 쓸지 정합니다. 비어 있으면 기본값은 영어입니다:

```bash
ocr config set language 中文
ocr config set language English
```

## 관련 문서 {#see-also}

- [빠른 시작](../quickstart/) — 최소 설정과 첫 리뷰.
- [CLI 레퍼런스](../cli-reference/) — review 명령이 받는 모든 플래그.
