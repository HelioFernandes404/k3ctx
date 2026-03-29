#!/usr/bin/env bash
#
# fix-lens-duplicates.sh - Limpa cache do Lens Desktop e valida kubeconfig
#
# Problema: Lens Desktop mostra contextos duplicados mesmo quando
# o ~/.kube/config está correto
#
# Solução:
# 1. Para o Lens Desktop
# 2. Limpa cache/storage do Lens
# 3. Valida kubeconfig
# 4. Reinicia Lens
#

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Lens Desktop config paths (Linux)
LENS_CONFIG_DIR="$HOME/.config/Lens"
LENS_CACHE_DIR="$HOME/.cache/Lens"

echo -e "${BLUE}=== Lens Desktop Duplicate Contexts Fix ===${NC}\n"

# 1. Verificar se Lens está rodando
echo -e "${YELLOW}[1/5] Verificando se Lens Desktop está rodando...${NC}"
if pgrep -f "Lens" > /dev/null; then
    echo -e "${YELLOW}⚠ Lens Desktop está rodando. Encerrando...${NC}"
    pkill -f "Lens" || true
    sleep 2
    echo -e "${GREEN}✓ Lens Desktop encerrado${NC}"
else
    echo -e "${GREEN}✓ Lens Desktop já está encerrado${NC}"
fi

# 2. Validar kubeconfig ANTES de limpar cache
echo -e "\n${YELLOW}[2/5] Validando ~/.kube/config...${NC}"
if kubectl config view --flatten > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Kubeconfig válido${NC}"

    # Mostrar estatísticas
    CONTEXTS_COUNT=$(kubectl config get-contexts -o name | wc -l)
    CLUSTERS_COUNT=$(kubectl config view -o jsonpath='{.clusters[*].name}' | wc -w)
    USERS_COUNT=$(kubectl config view -o jsonpath='{.users[*].name}' | wc -w)

    echo -e "  ${BLUE}→${NC} Contextos: $CONTEXTS_COUNT"
    echo -e "  ${BLUE}→${NC} Clusters: $CLUSTERS_COUNT"
    echo -e "  ${BLUE}→${NC} Usuários: $USERS_COUNT"
else
    echo -e "${RED}✗ Kubeconfig inválido! Abortando...${NC}"
    exit 1
fi

# 3. Fazer backup do kubeconfig (por segurança)
echo -e "\n${YELLOW}[3/5] Fazendo backup do kubeconfig...${NC}"
BACKUP_FILE="$HOME/.kube/config.backup-$(date +%Y%m%d-%H%M%S)"
cp "$HOME/.kube/config" "$BACKUP_FILE"
echo -e "${GREEN}✓ Backup criado: $BACKUP_FILE${NC}"

# 4. Limpar cache do Lens
echo -e "\n${YELLOW}[4/5] Limpando cache do Lens Desktop...${NC}"

if [ -d "$LENS_CACHE_DIR" ]; then
    echo -e "  ${BLUE}→${NC} Removendo $LENS_CACHE_DIR"
    rm -rf "$LENS_CACHE_DIR"
    echo -e "${GREEN}✓ Cache limpo${NC}"
else
    echo -e "${YELLOW}⚠ Diretório de cache não encontrado${NC}"
fi

if [ -d "$LENS_CONFIG_DIR/Local Storage" ]; then
    echo -e "  ${BLUE}→${NC} Removendo $LENS_CONFIG_DIR/Local Storage"
    rm -rf "$LENS_CONFIG_DIR/Local Storage"
    echo -e "${GREEN}✓ Local Storage limpo${NC}"
else
    echo -e "${YELLOW}⚠ Local Storage não encontrado${NC}"
fi

if [ -d "$LENS_CONFIG_DIR/Session Storage" ]; then
    echo -e "  ${BLUE}→${NC} Removendo $LENS_CONFIG_DIR/Session Storage"
    rm -rf "$LENS_CONFIG_DIR/Session Storage"
    echo -e "${GREEN}✓ Session Storage limpo${NC}"
else
    echo -e "${YELLOW}⚠ Session Storage não encontrado${NC}"
fi

# 5. Instruções finais
echo -e "\n${YELLOW}[5/5] Próximos passos:${NC}"
echo -e "  ${BLUE}1.${NC} Reinicie o Lens Desktop manualmente"
echo -e "  ${BLUE}2.${NC} O Lens vai recarregar todos os contextos do ~/.kube/config"
echo -e "  ${BLUE}3.${NC} Verifique se as duplicatas foram removidas"
echo -e "\n${GREEN}═══════════════════════════════════════${NC}"
echo -e "${GREEN}✓ Cache do Lens limpo com sucesso!${NC}"
echo -e "${GREEN}═══════════════════════════════════════${NC}\n"

echo -e "${BLUE}Dica:${NC} Se as duplicatas persistirem:"
echo -e "  • Verifique File → Preferences → Kubernetes → Kubeconfig Syncs"
echo -e "  • Certifique-se de que só há ~/.kube/config como fonte"
echo -e "  • Remova qualquer kubeconfig adicional configurado no Lens\n"
