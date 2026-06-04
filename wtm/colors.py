import re

RESET    = "\033[0m"
BOLD     = "\033[1m"
WHITE    = "\033[38;2;255;255;255m"
GREEN    = "\033[32m"
YELLOW   = "\033[33m"
RED      = "\033[31m"
DIM      = "\033[2m"
CYAN     = "\033[36m"
BLUE     = "\033[34m"
UNDERLINE = "\033[4m"
WHITE_FG = "\033[97m"
BG_GREEN  = "\033[48;5;28m"
BG_PURPLE = "\033[48;5;93m"
BG_RED    = "\033[48;5;160m"
BG_GRAY   = "\033[48;5;240m"

_ANSI_RE = re.compile(r"\033\[[^m]*m")
