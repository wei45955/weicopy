import React, { createContext, useContext, useState, useEffect } from 'react';
import axios from 'axios';

const AuthContext = createContext();

export const useAuth = () => useContext(AuthContext);

export const AuthProvider = ({ children }) => {
  const [currentUser, setCurrentUser] = useState(null);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    // 检查本地存储中是否有令牌
    const token = localStorage.getItem('token');
    if (token) {
      // 设置axios默认头部
      axios.defaults.headers.common['Authorization'] = `Bearer ${token}`;
      // 验证令牌
      checkAuthStatus();
    } else {
      setLoading(false);
    }
  }, []);

  // 验证当前认证状态
  const checkAuthStatus = async () => {
    try {
      const response = await axios.get('/api/auth/me');
      setCurrentUser(response.data);
      setIsAuthenticated(true);
    } catch (err) {
      // 如果令牌无效，清除本地存储
      localStorage.removeItem('token');
      delete axios.defaults.headers.common['Authorization'];
    } finally {
      setLoading(false);
    }
  };

  // 登录函数
  const login = async (username, password) => {
    try {
      setError('');
      const response = await axios.post('/api/auth/login', { username, password });
      const { token, user } = response.data;
      
      // 保存令牌到本地存储
      localStorage.setItem('token', token);
      
      // 设置axios默认头部
      axios.defaults.headers.common['Authorization'] = `Bearer ${token}`;
      
      setCurrentUser(user);
      setIsAuthenticated(true);
      return true;
    } catch (err) {
      setError(err.response?.data?.message || '登录失败');
      return false;
    }
  };

  // 注册函数
  const register = async (username, password) => {
    try {
      setError('');
      await axios.post('/api/auth/register', { username, password });
      return true;
    } catch (err) {
      setError(err.response?.data?.message || '注册失败');
      return false;
    }
  };

  // 登出函数
  const logout = () => {
    localStorage.removeItem('token');
    delete axios.defaults.headers.common['Authorization'];
    setCurrentUser(null);
    setIsAuthenticated(false);
  };

  const value = {
    currentUser,
    isAuthenticated,
    loading,
    error,
    login,
    register,
    logout,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};