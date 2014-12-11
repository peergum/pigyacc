/*
 * Lexical and Grammar Definitions for PigLex/PigYacc file
 */

%token COMMENT_START
%token COMMENT_END
%token COMMENT
%token PERCENT
%token TOKEN_DEF
%token STATE_DEF
%token LEX
%token YACC
%token INCLUDE
%token ONLY
%token EXCEPT
%token ACTION_START
%token ACTION_END
%token RETURN
%token STATE
%token VALUE
%token STRING
%token SEPARATOR

%state _COMMENT
%state _PERCENT
%state _STRING
%state _LEX_RULE
%state _LEX_ACTION
%state _ACTION_BLOCK
%state _YACC_RULE

/*
 * ---------
 * LEX RULES
 * ---------
 */

%lex

// ----- init mode
%except _STRING

/*      {
    state _COMMENT
    return COMMENT_START
    }

(//|#)[^\n]+      return COMMENT

\%       state _PERCENT

// ----- command mode
%only _PERCENT

only    return ONLY
except  return EXCEPT
lex     {
    state _LEX_RULE
    return LEX
    }

yacc    {
    state _YACC_RULE
    return YACC
    }
token   return TOKEN
include return INCLUDE
state   return STATE
"       state _STRING
\n      pop
[, ]    RETURN SEPARATOR
[^"\n, ]+   return VALUE

// ----- string state
%only _STRING

[^"]*   return STRING
"       pop

// ----- lex rule mode is started by a grammar rule
%only _LEX_RULE

[^\t\n]+   return REGEXP
\t+     state _LEX_ACTION

// ----- lex action mode starts after a tab
%only _LEX_ACTION

{   {
    state _ACTION_BLOCK
    return ACTION_START
    }
[ \t\n]+
state   return STATE
return  return RETURN
\$[0-9]+    return VALUE
}   {
    state _LEX_RULE
}

/*
 * -------------
 * GRAMMAR RULES
 * -------------
 */

%yacc

rules_file:
    token_definitions state_definitions includes rules

token_definitions:
    EMPTY
    | token_definitions token

token:
    TOKEN_DEF names

state_definitions:
    EMPTY
    | state_definitions state

state:
    STATE_DEF names

names:
    VALUE
    | names SEPARATOR VALUE

includes:
    EMPY
    | includes include

include:
    INCLUDE strings

strings:
    STRING
    | strings SEPARATOR STRING

rules:
    EMPTY
    | rules LEX lex_rules
    | rules YACC yacc_rules

lex_rules:
    lex_rule
    | lex_rules lex_rule

lex_rule:
    REGEXP
    | REGEXP lex_action

lex_action:
    lex_simple_actions
    | lex_action_block

lex_simple_actions:
    ACTION
    | lex_simple_actions ACTION

lex_action_block:
    ACTION_START lex_simple_actions ACTION_END

