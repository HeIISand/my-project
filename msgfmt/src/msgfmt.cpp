#include "msgfmt.h"

namespace msgfmt {

static bool is_special_char(char c) {
    return c == '*' || c == '_' || c == '`' ||
           c == '[' || c == ']' || c == '(' ||
           c == ')' || c == '\\';
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
        uint32_t cp = 0;
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

}
