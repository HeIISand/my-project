#include "msgfmt.h"
#include <algorithm>

namespace msgfmt {

bool is_special_char(char c) {
    return c == '*' || c == '_' || c == '`' ||
           c == '[' || c == ']' || c == '(' ||
           c == ')' || c == '\\';
}

bool is_special_cp(long long cp) {
    return cp == '*' || cp == '_' || cp == '`' ||
           cp == '[' || cp == ']' || cp == '(' ||
           cp == ')' || cp == '\\';
}

bool is_space_cp(long long cp) {
    if (cp == ' ' || cp == '\t' || cp == '\n' || cp == '\r' || cp == '\v' || cp == '\f') {
        return true;
    }
    if (cp == 0x00A0 || cp == 0x1680 || cp == 0x2028 || cp == 0x2029 ||
        cp == 0x202F || cp == 0x205F || cp == 0x3000) {
        return true;
    }
    return cp >= 0x2000 && cp <= 0x200A;
}

bool is_word_cp(long long cp) {
    if (cp >= '0' && cp <= '9') return true;
    if (cp >= 'a' && cp <= 'z') return true;
    if (cp >= 'A' && cp <= 'Z') return true;
    if (cp >= 0x00C0 && cp <= 0x02AF && cp != 0x00D7 && cp != 0x00F7) return true;
    if (cp >= 0x0370 && cp <= 0x052F) return true;
    return false;
}

size_t utf16_units(long long cp) {
    return cp > 0xFFFF ? 2 : 1;
}

bool is_cont(unsigned char b) {
    return (b & 0b11000000) == 0b10000000;
}

bool decode_next(const std::string& s, size_t pos, long long& cp, size_t& len) {
    unsigned char b0 = static_cast<unsigned char>(s[pos]);

    if (b0 <= 0b01111111) {
        cp = b0;
        len = 1;
        return true;
    }

    if ((b0 & 0b11100000) == 0b11000000) {
        if (pos + 1 >= s.size()) return false;
        unsigned char b1 = static_cast<unsigned char>(s[pos + 1]);
        if (!is_cont(b1)) return false;

        cp = ((b0 & 0b00011111) << 6) | (b1 & 0b00111111);
        len = 2;
        return cp >= 0x80;
    }

    if ((b0 & 0b11110000) == 0b11100000) {
        if (pos + 2 >= s.size()) return false;
        unsigned char b1 = static_cast<unsigned char>(s[pos + 1]);
        unsigned char b2 = static_cast<unsigned char>(s[pos + 2]);
        if (!is_cont(b1) || !is_cont(b2)) return false;

        cp = ((b0 & 0b00001111) << 12) | ((b1 & 0b00111111) << 6) | (b2 & 0b00111111);
        len = 3;
        return cp >= 0x800 && (cp < 0xD800 || cp > 0xDFFF);
    }

    if ((b0 & 0b11111000) == 0b11110000) {
        if (pos + 3 >= s.size()) return false;
        unsigned char b1 = static_cast<unsigned char>(s[pos + 1]);
        unsigned char b2 = static_cast<unsigned char>(s[pos + 2]);
        unsigned char b3 = static_cast<unsigned char>(s[pos + 3]);
        if (!is_cont(b1) || !is_cont(b2) || !is_cont(b3)) return false;

        cp = ((b0 & 0b00000111) << 18) | ((b1 & 0b00111111) << 12) | ((b2 & 0b00111111) << 6) | (b3 & 0b00111111);
        len = 4;
        return cp >= 0x10000 && cp <= 0x10FFFF;
    }

    return false;
}

bool is_valid_utf8(const std::string& input) {
    size_t i = 0;
    while (i < input.size()) {
        long long cp = 0;
        size_t len = 0;
        if (!decode_next(input, i, cp, len)) {
            return false;
        }
        i += len;
    }
    return true;
}

std::string escape(const std::string& input) {
    std::string out;
    out.reserve(input.size());
    for (char c : input) {
        if (is_special_char(c)) {
            out.push_back('\\');
        }
        out.push_back(c);
    }
    return out;
}

struct CharToken {
    long long cp;
    size_t byte_pos;
    size_t byte_len;
};

struct ActiveStyles {
    bool bold = false;
    bool italic = false;
    bool code = false;
    bool link = false;
};

struct RawRange {
    Range range;
    size_t open_order;
};

bool find_close_code(const std::vector<CharToken>& chars, size_t open_pos, size_t end, size_t& close_pos) {
    if (open_pos + 1 >= end) return false;
    if (is_space_cp(chars[open_pos + 1].cp) || chars[open_pos + 1].cp == '`') return false;

    for (size_t j = open_pos + 2; j < end; ++j) {
        if (chars[j].cp == '`' && !is_space_cp(chars[j - 1].cp)) {
            close_pos = j;
            return true;
        }
    }
    return false;
}

bool match_link(const std::vector<CharToken>& chars, size_t open_pos, size_t end,
                size_t& close_bracket, size_t& close_paren) {
    if (open_pos + 1 >= end) return false;
    if (is_space_cp(chars[open_pos + 1].cp) || chars[open_pos + 1].cp == ']') return false;

    bool found_bracket = false;
    size_t k = open_pos + 1;
    while (k < end) {
        if (chars[k].cp == '\\' && k + 1 < end && is_special_cp(chars[k + 1].cp)) {
            if (chars[k + 1].cp == ']') return false;
            k += 2;
            continue;
        }
        if (chars[k].cp == ']') {
            close_bracket = k;
            found_bracket = true;
            break;
        }
        k += 1;
    }

    if (!found_bracket || close_bracket <= open_pos + 1) return false;
    if (is_space_cp(chars[close_bracket - 1].cp)) return false;

    if (close_bracket + 1 >= end || chars[close_bracket + 1].cp != '(') return false;

    size_t url_start = close_bracket + 2;
    bool found_paren = false;
    for (size_t p = url_start; p < end; ++p) {
        if (chars[p].cp == ')') {
            close_paren = p;
            found_paren = true;
            break;
        }
        if (is_space_cp(chars[p].cp)) {
            return false;
        }
    }

    if (!found_paren || close_paren <= url_start) return false;
    return true;
}

bool find_close_bold(const std::vector<CharToken>& chars, size_t open_pos, size_t end,
                     ActiveStyles active, size_t& close_pos) {
    size_t content_start = open_pos + 2;
    if (content_start >= end || is_space_cp(chars[content_start].cp)) return false;

    size_t j = content_start;
    while (j + 1 < end) {
        if (chars[j].cp == '\\' && j + 1 < end && is_special_cp(chars[j + 1].cp)) {
            j += 2;
            continue;
        }
        if (!active.code && chars[j].cp == '`') {
            size_t c_close = 0;
            if (find_close_code(chars, j, end, c_close)) {
                j = c_close + 1;
                continue;
            }
        }
        if (!active.link && chars[j].cp == '[') {
            size_t cb = 0;
            size_t cp = 0;
            if (match_link(chars, j, end, cb, cp)) {
                j = cp + 1;
                continue;
            }
        }
        if (j > content_start && chars[j].cp == '*' && chars[j + 1].cp == '*' && !is_space_cp(chars[j - 1].cp)) {
            close_pos = j;
            return true;
        }
        j += 1;
    }
    return false;
}

bool find_close_italic(const std::vector<CharToken>& chars, size_t open_pos, size_t end,
                       ActiveStyles active, size_t& close_pos) {
    if (open_pos > 0 && is_word_cp(chars[open_pos - 1].cp)) return false;
    size_t content_start = open_pos + 1;
    if (content_start >= end || is_space_cp(chars[content_start].cp)) return false;

    size_t j = content_start;
    while (j < end) {
        if (chars[j].cp == '\\' && j + 1 < end && is_special_cp(chars[j + 1].cp)) {
            j += 2;
            continue;
        }
        if (!active.code && chars[j].cp == '`') {
            size_t c_close = 0;
            if (find_close_code(chars, j, end, c_close)) {
                j = c_close + 1;
                continue;
            }
        }
        if (!active.link && chars[j].cp == '[') {
            size_t cb = 0;
            size_t cp = 0;
            if (match_link(chars, j, end, cb, cp)) {
                j = cp + 1;
                continue;
            }
        }
        if (j > content_start && chars[j].cp == '_' && !is_space_cp(chars[j - 1].cp)) {
            if (j + 1 >= chars.size() || !is_word_cp(chars[j + 1].cp)) {
                close_pos = j;
                return true;
            }
        }
        j += 1;
    }
    return false;
}

struct ParserState {
    const std::string& input;
    const std::vector<CharToken>& chars;
    std::string plain_text;
    size_t utf16_offset = 0;
    size_t next_order = 0;
    std::vector<RawRange> raw_ranges;

    void append_token(const CharToken& tok) {
        plain_text.append(input, tok.byte_pos, tok.byte_len);
        utf16_offset += utf16_units(tok.cp);
    }

    void parse_slice(size_t start, size_t end, ActiveStyles active) {
        size_t i = start;
        while (i < end) {
            if (chars[i].cp == '\\' && i + 1 < end && is_special_cp(chars[i + 1].cp)) {
                append_token(chars[i + 1]);
                i += 2;
                continue;
            }

            if (!active.code && chars[i].cp == '`') {
                size_t close_pos = 0;
                if (find_close_code(chars, i, end, close_pos)) {
                    size_t order = next_order++;
                    size_t start_u16 = utf16_offset;
                    for (size_t k = i + 1; k < close_pos; ++k) {
                        append_token(chars[k]);
                    }
                    size_t len_u16 = utf16_offset - start_u16;
                    raw_ranges.push_back({Range{"code", start_u16, len_u16, ""}, order});
                    i = close_pos + 1;
                    continue;
                }
            }

            if (!active.bold && i + 1 < end && chars[i].cp == '*' && chars[i + 1].cp == '*') {
                size_t close_pos = 0;
                if (find_close_bold(chars, i, end, active, close_pos)) {
                    size_t order = next_order++;
                    size_t start_u16 = utf16_offset;
                    ActiveStyles next_active = active;
                    next_active.bold = true;
                    parse_slice(i + 2, close_pos, next_active);
                    size_t len_u16 = utf16_offset - start_u16;
                    raw_ranges.push_back({Range{"bold", start_u16, len_u16, ""}, order});
                    i = close_pos + 2;
                    continue;
                }
            }

            if (!active.italic && chars[i].cp == '_') {
                size_t close_pos = 0;
                if (find_close_italic(chars, i, end, active, close_pos)) {
                    size_t order = next_order++;
                    size_t start_u16 = utf16_offset;
                    ActiveStyles next_active = active;
                    next_active.italic = true;
                    parse_slice(i + 1, close_pos, next_active);
                    size_t len_u16 = utf16_offset - start_u16;
                    raw_ranges.push_back({Range{"italic", start_u16, len_u16, ""}, order});
                    i = close_pos + 1;
                    continue;
                }
            }

            if (!active.link && chars[i].cp == '[') {
                size_t close_bracket = 0;
                size_t close_paren = 0;
                if (match_link(chars, i, end, close_bracket, close_paren)) {
                    size_t url_byte_start = chars[close_bracket + 2].byte_pos;
                    size_t url_byte_end = chars[close_paren].byte_pos;
                    std::string url = input.substr(url_byte_start, url_byte_end - url_byte_start);

                    size_t order = next_order++;
                    size_t start_u16 = utf16_offset;
                    ActiveStyles next_active = active;
                    next_active.link = true;
                    parse_slice(i + 1, close_bracket, next_active);
                    size_t len_u16 = utf16_offset - start_u16;
                    raw_ranges.push_back({Range{"link", start_u16, len_u16, url}, order});
                    i = close_paren + 1;
                    continue;
                }
            }

            append_token(chars[i]);
            i += 1;
        }
    }
};

ParsedResult parse(const std::string& input) {
    std::vector<CharToken> chars;
    size_t pos = 0;
    while (pos < input.size()) {
        long long cp = 0;
        size_t len = 0;
        if (!decode_next(input, pos, cp, len)) {
            break;
        }
        chars.push_back({cp, pos, len});
        pos += len;
    }

    ParserState state{input, chars, "", 0, 0, {}};
    state.parse_slice(0, chars.size(), ActiveStyles{});

    std::sort(state.raw_ranges.begin(), state.raw_ranges.end(), [](const RawRange& a, const RawRange& b) {
        if (a.range.offset != b.range.offset) {
            return a.range.offset < b.range.offset;
        }
        if (a.range.length != b.range.length) {
            return a.range.length > b.range.length;
        }
        return a.open_order < b.open_order;
    });

    ParsedResult res;
    res.text = std::move(state.plain_text);
    res.ranges.reserve(state.raw_ranges.size());
    for (auto& rr : state.raw_ranges) {
        res.ranges.push_back(std::move(rr.range));
    }
    return res;
}

std::string json_escape(const std::string& s) {
    std::string out;
    out.reserve(s.size());
    const char* hex = "0123456789abcdef";
    for (unsigned char c : s) {
        if (c == '"') {
            out += "\\\"";
        } else if (c == '\\') {
            out += "\\\\";
        } else if (c == '\b') {
            out += "\\b";
        } else if (c == '\f') {
            out += "\\f";
        } else if (c == '\n') {
            out += "\\n";
        } else if (c == '\r') {
            out += "\\r";
        } else if (c == '\t') {
            out += "\\t";
        } else if (c < 0x20) {
            out += "\\u00";
            out.push_back(hex[(c >> 4) & 0x0F]);
            out.push_back(hex[c & 0x0F]);
        } else {
            out.push_back(static_cast<char>(c));
        }
    }
    return out;
}

std::string to_json(const ParsedResult& result) {
    std::string out = "{\"text\": \"";
    out += json_escape(result.text);
    out += "\", \"ranges\": [";
    for (size_t i = 0; i < result.ranges.size(); ++i) {
        if (i > 0) {
            out += ", ";
        }
        const Range& r = result.ranges[i];
        out += "{\"style\": \"";
        out += json_escape(r.style);
        out += "\", \"offset\": ";
        out += std::to_string(r.offset);
        out += ", \"length\": ";
        out += std::to_string(r.length);
        out += ", \"url\": \"";
        out += json_escape(r.url);
        out += "\"}";
    }
    out += "]}";
    return out;
}

}
